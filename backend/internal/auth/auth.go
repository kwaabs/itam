package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"itam/internal/config"
	"itam/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
	accessTokenTTL  = time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

var (
	// ErrInvalidCredentials covers both "no such user" and "wrong password" -
	// deliberately not distinguished, so a failed login can't be used to
	// enumerate valid emails.
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// TokenPair is the response shape for local login/refresh, matching the
// access_token/refresh_token/user fields GoTrue's responses use today so
// frontend/mobile clients need minimal changes when they switch over.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	User         *UserInfo `json:"user"`
}

type UserInfo struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

type ctxKey string

const principalKey ctxKey = "principal"

// Principal is the authenticated caller derived from a GoTrue JWT.
type Principal struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	IsSuperuser bool      `json:"is_superuser"`
}

// Service validates GoTrue tokens and provisions app profiles just-in-time.
type Service struct {
	cfg         config.Config
	db          *bun.DB
	log         *slog.Logger
	provisioned sync.Map // userID -> struct{}
}

func New(cfg config.Config, db *bun.DB, log *slog.Logger) *Service {
	return &Service{cfg: cfg, db: db, log: log}
}

// Middleware authenticates the request and attaches the Principal to context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearer(r)
		if raw == "" {
			http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}
		claims, err := s.parse(raw)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		sub, _ := claims["sub"].(string)
		uid, err := uuid.Parse(sub)
		if err != nil {
			http.Error(w, `{"error":"invalid subject"}`, http.StatusUnauthorized)
			return
		}
		email, _ := claims["email"].(string)
		p := &Principal{UserID: uid, Email: email}

		if err := s.ensureProfile(r.Context(), p); err != nil {
			s.log.Error("profile provisioning failed", "err", err, "user", uid)
		}

		ctx := context.WithValue(r.Context(), principalKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) parse(raw string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.GoTrueJWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// ensureProfile upserts the iam profile on first sight of a user and, if the
// user is the configured admin, flags superuser + grants the admin role.
func (s *Service) ensureProfile(ctx context.Context, p *Principal) error {
	isAdmin := p.Email != "" && strings.EqualFold(p.Email, s.cfg.AdminEmail)
	p.IsSuperuser = isAdmin

	if _, seen := s.provisioned.Load(p.UserID); seen {
		return nil
	}

	profile := &domain.UserProfile{
		UserID:      p.UserID,
		Email:       p.Email,
		IsActive:    true,
		IsSuperuser: isAdmin,
	}
	_, err := s.db.NewInsert().Model(profile).
		On("CONFLICT (user_id) DO UPDATE").
		Set("email = EXCLUDED.email").
		Set("updated_at = now()").
		Exec(ctx)
	if err != nil {
		return err
	}

	if isAdmin {
		if _, err := s.db.NewUpdate().Model((*domain.UserProfile)(nil)).
			Set("is_superuser = ?", true).
			Where("user_id = ?", p.UserID).Exec(ctx); err != nil {
			return err
		}
		if err := s.ensureAdminGrant(ctx, p.UserID); err != nil {
			return err
		}
	}

	// also keep the superuser flag in the principal accurate
	var prof domain.UserProfile
	if err := s.db.NewSelect().Model(&prof).Where("user_id = ?", p.UserID).Scan(ctx); err == nil {
		p.IsSuperuser = prof.IsSuperuser
	}

	s.provisioned.Store(p.UserID, struct{}{})
	return nil
}

func (s *Service) ensureAdminGrant(ctx context.Context, uid uuid.UUID) error {
	n, err := s.db.NewSelect().Model((*domain.RoleGrant)(nil)).
		Join("JOIN iam.roles r ON r.id = rg.role_id").
		Where("rg.user_id = ?", uid).
		Where("r.key = ?", "admin").
		Where("rg.scope_type = ?", "global").
		Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = s.db.NewRaw(
		`INSERT INTO iam.role_grants (user_id, role_id, scope_type)
		 SELECT ?, r.id, 'global' FROM iam.roles r WHERE r.key = 'admin'`, uid).Exec(ctx)
	return err
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return ""
}

// FromContext returns the authenticated principal.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

// BootstrapAdmin idempotently creates the configured admin user directly in
// iam.user_profiles so the instance is usable out of the box, and grants it
// the admin role. Safe to call on every startup.
func (s *Service) BootstrapAdmin(ctx context.Context) {
	if s.cfg.AdminEmail == "" || s.cfg.AdminPassword == "" {
		return
	}
	hash, err := HashPassword(s.cfg.AdminPassword)
	if err != nil {
		s.log.Error("could not hash bootstrap admin password", "err", err)
		return
	}

	profile := &domain.UserProfile{
		UserID:       uuid.New(),
		Email:        s.cfg.AdminEmail,
		PasswordHash: hash,
		IsSuperuser:  true,
		IsActive:     true,
	}
	res, err := s.db.NewInsert().Model(profile).On("CONFLICT (email) DO NOTHING").Exec(ctx)
	if err != nil {
		s.log.Error("bootstrap admin insert failed", "err", err)
		return
	}
	userID := profile.UserID
	if n, _ := res.RowsAffected(); n == 0 {
		s.log.Info("bootstrap admin user already exists", "email", s.cfg.AdminEmail)
		var existing domain.UserProfile
		if err := s.db.NewSelect().Model(&existing).Where("email = ?", s.cfg.AdminEmail).Scan(ctx); err != nil {
			s.log.Error("could not look up existing bootstrap admin", "err", err)
			return
		}
		userID = existing.UserID
	} else {
		s.log.Info("bootstrap admin user created", "email", s.cfg.AdminEmail)
	}

	if err := s.ensureAdminGrant(ctx, userID); err != nil {
		s.log.Error("bootstrap admin role grant failed", "err", err)
	}
}

// IssueAccessToken mints an app-issued JWT for a local (password) login,
// signed with the same shared secret GoTrue tokens use today so it validates
// through the exact same Middleware/parse() path without any changes there.
// Shaped like GoTrue's claims (sub, email, role) so downstream code that
// reads the JWT doesn't need to know who issued it.
func (s *Service) IssueAccessToken(userID uuid.UUID, email string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),
		"email": email,
		"role":  "authenticated",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(ttl).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.cfg.GoTrueJWTSecret))
}

// Login verifies email/password against iam.user_profiles and, on success,
// issues a fresh access/refresh token pair.
func (s *Service) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	var profile domain.UserProfile
	err := s.db.NewSelect().Model(&profile).
		Where("email = ?", email).
		Where("is_active = true").
		Scan(ctx)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if profile.PasswordHash == "" || !VerifyPassword(profile.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return s.issuePair(ctx, profile.UserID, profile.Email)
}

// Refresh rotates a refresh token: the presented token is revoked and a new
// pair is issued, so a stolen-but-already-used token stops working.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	var rt domain.RefreshToken
	err := s.db.NewSelect().Model(&rt).Where("token_hash = ?", hashToken(refreshToken)).Scan(ctx)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	if rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		return nil, ErrInvalidRefreshToken
	}
	if _, err := s.db.NewUpdate().Model((*domain.RefreshToken)(nil)).
		Set("revoked_at = now()").Where("id = ?", rt.ID).Exec(ctx); err != nil {
		return nil, err
	}

	var profile domain.UserProfile
	if err := s.db.NewSelect().Model(&profile).
		Where("user_id = ?", rt.UserID).
		Where("is_active = true").
		Scan(ctx); err != nil {
		return nil, ErrInvalidRefreshToken
	}
	return s.issuePair(ctx, profile.UserID, profile.Email)
}

// Logout revokes a refresh token. Idempotent: revoking an already-revoked or
// unknown token is not an error.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	_, err := s.db.NewUpdate().Model((*domain.RefreshToken)(nil)).
		Set("revoked_at = now()").
		Where("token_hash = ?", hashToken(refreshToken)).
		Where("revoked_at IS NULL").
		Exec(ctx)
	return err
}

func (s *Service) issuePair(ctx context.Context, userID uuid.UUID, email string) (*TokenPair, error) {
	access, err := s.IssueAccessToken(userID, email, accessTokenTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}
	rt := &domain.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(refresh),
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if _, err := s.db.NewInsert().Model(rt).Exec(ctx); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		User:         &UserInfo{ID: userID, Email: email},
	}, nil
}

// generateOpaqueToken returns a random URL-safe refresh token. Opaque (not a
// JWT) and stored server-side hashed, mirroring GoTrue's own refresh tokens -
// unlike access tokens, revocation has to be checkable without decoding.
func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
