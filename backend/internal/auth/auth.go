package auth

import (
	"bytes"
	"context"
	"encoding/json"
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

// BootstrapAdmin best-effort creates the configured admin user in GoTrue so the
// instance is usable out of the box. Idempotent; tolerant of GoTrue not being
// ready yet (retries a few times). Superuser/role wiring happens via JIT
// provisioning on first login.
func (s *Service) BootstrapAdmin(ctx context.Context) {
	if s.cfg.AdminEmail == "" || s.cfg.AdminPassword == "" {
		return
	}
	token, err := s.serviceToken()
	if err != nil {
		s.log.Warn("could not mint service token for bootstrap", "err", err)
		return
	}
	body, _ := json.Marshal(map[string]any{
		"email":         s.cfg.AdminEmail,
		"password":      s.cfg.AdminPassword,
		"email_confirm": true,
	})
	url := strings.TrimRight(s.cfg.GoTrueURL, "/") + "/admin/users"

	for attempt := 1; attempt <= 10; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		status := resp.StatusCode
		_ = resp.Body.Close()
		switch {
		case status >= 200 && status < 300:
			s.log.Info("bootstrap admin user created in GoTrue", "email", s.cfg.AdminEmail)
			return
		case status == http.StatusConflict || status == http.StatusUnprocessableEntity:
			s.log.Info("bootstrap admin user already exists", "email", s.cfg.AdminEmail)
			return
		default:
			s.log.Warn("bootstrap admin attempt failed", "status", status, "attempt", attempt)
			time.Sleep(3 * time.Second)
		}
	}
}

// serviceToken mints a short-lived service_role JWT GoTrue accepts on its admin
// API (it is signed with the shared secret).
func (s *Service) serviceToken() (string, error) {
	claims := jwt.MapClaims{
		"role": "service_role",
		"sub":  "00000000-0000-0000-0000-000000000000",
		"iss":  "itam-bootstrap",
		"exp":  time.Now().Add(10 * time.Minute).Unix(),
		"iat":  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.cfg.GoTrueJWTSecret))
}
