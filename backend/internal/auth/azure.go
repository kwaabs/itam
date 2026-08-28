package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const azureStateTTL = 10 * time.Minute

var (
	ErrAzureNotConfigured = errors.New("azure sso not configured")
	ErrAzureInvalidState  = errors.New("invalid or expired sso state")
)

// azureState round-trips the "where do I send the browser back to" (a web
// origin, or the itam:// deep link on native) through the Entra AD redirect,
// HMAC-signed with the shared JWT secret so it needs no server-side storage.
type azureState struct {
	RedirectTo string `json:"redirect_to"`
	Nonce      string `json:"nonce"`
	Exp        int64  `json:"exp"`
}

func (s *Service) signAzureState(redirectTo string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	st := azureState{
		RedirectTo: redirectTo,
		Nonce:      hex.EncodeToString(nonce),
		Exp:        time.Now().Add(azureStateTTL).Unix(),
	}
	payload, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	return b64 + "." + s.signAzureStatePayload(b64), nil
}

func (s *Service) verifyAzureState(state string) (*azureState, error) {
	b64, sig, ok := strings.Cut(state, ".")
	if !ok {
		return nil, ErrAzureInvalidState
	}
	if !hmac.Equal([]byte(sig), []byte(s.signAzureStatePayload(b64))) {
		return nil, ErrAzureInvalidState
	}
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil, ErrAzureInvalidState
	}
	var st azureState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, ErrAzureInvalidState
	}
	if time.Now().Unix() > st.Exp {
		return nil, ErrAzureInvalidState
	}
	return &st, nil
}

func (s *Service) signAzureStatePayload(b64 string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte(b64))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// AzureStateRedirectTo best-effort recovers the redirect_to carried in a
// (possibly expired, possibly tampered) state value, or "" if it can't be
// verified. Used to send the user back to a page that can show an error
// instead of a bare API error response.
func (s *Service) AzureStateRedirectTo(state string) string {
	st, err := s.verifyAzureState(state)
	if err != nil {
		return ""
	}
	return st.RedirectTo
}

// AzureStartURL builds the Entra ID authorization URL to send the browser (or
// native system browser) to. redirectTo is where the callback should send the
// user back to afterward - a web origin, or the itam:// deep link on native.
func (s *Service) AzureStartURL(ctx context.Context, redirectTo string) (string, error) {
	if s.set.GetOr(ctx, "azure_enabled", "false") != "true" {
		return "", ErrAzureNotConfigured
	}
	tenant := s.set.GetOr(ctx, "azure_tenant", "")
	clientID := s.set.GetOr(ctx, "azure_client_id", "")
	redirectURI := s.set.GetOr(ctx, "azure_redirect_url", "")
	if tenant == "" || clientID == "" || redirectURI == "" {
		return "", ErrAzureNotConfigured
	}
	state, err := s.signAzureState(redirectTo)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("response_mode", "query")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s",
		url.PathEscape(tenant), q.Encode()), nil
}

// AzureCallback completes the flow: verifies state, exchanges the code for an
// id_token, verifies its signature/claims against Microsoft's published keys,
// upserts the local user profile by email, and issues our own token pair.
// Returns the pair and the redirectTo carried through the state.
func (s *Service) AzureCallback(ctx context.Context, code, state string) (*TokenPair, string, error) {
	st, err := s.verifyAzureState(state)
	if err != nil {
		return nil, "", err
	}

	tenant := s.set.GetOr(ctx, "azure_tenant", "")
	clientID := s.set.GetOr(ctx, "azure_client_id", "")
	clientSecret := s.set.GetOr(ctx, "azure_client_secret", "")
	redirectURI := s.set.GetOr(ctx, "azure_redirect_url", "")
	if tenant == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		return nil, st.RedirectTo, ErrAzureNotConfigured
	}

	oidcCfg, keys, err := azureKeys.get(ctx, tenant)
	if err != nil {
		return nil, st.RedirectTo, fmt.Errorf("fetching azure oidc metadata: %w", err)
	}
	idToken, err := exchangeAzureCode(ctx, oidcCfg.TokenEndpoint, clientID, clientSecret, code, redirectURI)
	if err != nil {
		return nil, st.RedirectTo, err
	}
	email, err := verifyAzureIDToken(idToken, keys, clientID, oidcCfg.Issuer)
	if err != nil {
		return nil, st.RedirectTo, err
	}

	userID, err := s.upsertAzureUser(ctx, email)
	if err != nil {
		return nil, st.RedirectTo, err
	}
	pair, err := s.issuePair(ctx, userID, email)
	if err != nil {
		return nil, st.RedirectTo, err
	}
	return pair, st.RedirectTo, nil
}

// upsertAzureUser finds-or-creates the local profile for an Azure-verified
// email. Uses INSERT ... ON CONFLICT ... RETURNING rather than SELECT-then-
// insert so a concurrent first login from the same account can't create two
// profiles, and so the actually-winning user_id is always the one returned
// (not whichever random uuid.New() happened to be generated locally).
func (s *Service) upsertAzureUser(ctx context.Context, email string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := s.db.NewRaw(`
		INSERT INTO iam.user_profiles (user_id, email, is_active)
		VALUES (?, ?, true)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING user_id
	`, uuid.New(), email).Scan(ctx, &userID)
	return userID, err
}

// ---- OIDC discovery / JWKS / token exchange --------------------------------

type azureOIDCConfig struct {
	TokenEndpoint string `json:"token_endpoint"`
	JWKSURI       string `json:"jwks_uri"`
	Issuer        string `json:"issuer"`
}

type azureJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// azureKeyCacheStore caches the OIDC discovery document + JWKS per tenant.
// Microsoft rotates signing keys infrequently; fetching them on every login
// would add avoidable latency and load to every SSO attempt.
type azureKeyCacheStore struct {
	mu      sync.Mutex
	tenant  string
	fetched time.Time
	cfg     azureOIDCConfig
	keys    map[string]*rsa.PublicKey
}

var azureKeys azureKeyCacheStore

const azureKeyCacheTTL = time.Hour

func (c *azureKeyCacheStore) get(ctx context.Context, tenant string) (azureOIDCConfig, map[string]*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tenant == tenant && time.Since(c.fetched) < azureKeyCacheTTL && len(c.keys) > 0 {
		return c.cfg, c.keys, nil
	}
	cfg, err := fetchAzureOIDCConfig(ctx, tenant)
	if err != nil {
		return azureOIDCConfig{}, nil, err
	}
	keys, err := fetchAzureJWKS(ctx, cfg.JWKSURI)
	if err != nil {
		return azureOIDCConfig{}, nil, err
	}
	c.tenant, c.fetched, c.cfg, c.keys = tenant, time.Now(), cfg, keys
	return cfg, keys, nil
}

func fetchAzureOIDCConfig(ctx context.Context, tenant string) (azureOIDCConfig, error) {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0/.well-known/openid-configuration",
		url.PathEscape(tenant))
	var cfg azureOIDCConfig
	if err := fetchJSON(ctx, endpoint, &cfg); err != nil {
		return azureOIDCConfig{}, err
	}
	return cfg, nil
}

func fetchAzureJWKS(ctx context.Context, jwksURI string) (map[string]*rsa.PublicKey, error) {
	var doc struct {
		Keys []azureJWK `json:"keys"`
	}
	if err := fetchJSON(ctx, jwksURI, &doc); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pk, err := parseRSAJWK(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pk
	}
	if len(keys) == 0 {
		return nil, errors.New("no usable RSA keys in jwks")
	}
	return keys, nil
}

func parseRSAJWK(k azureJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

func fetchJSON(ctx context.Context, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, endpoint)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// exchangeAzureCode swaps an authorization code for an id_token at Entra's
// token endpoint.
func exchangeAzureCode(ctx context.Context, tokenEndpoint, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("scope", "openid email profile")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure token exchange failed (%d): %s", resp.StatusCode, string(body))
	}
	var out struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.IDToken == "" {
		return "", errors.New("azure token response had no id_token")
	}
	return out.IDToken, nil
}

// verifyAzureIDToken verifies the id_token's RS256 signature against
// Microsoft's published keys and checks audience/issuer/expiry, returning the
// verified email claim.
func verifyAzureIDToken(idToken string, keys map[string]*rsa.PublicKey, clientID, issuer string) (string, error) {
	token, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown signing key %q", kid)
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(clientID), jwt.WithIssuer(issuer))
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid azure id_token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid azure id_token claims")
	}
	email, _ := claims["email"].(string)
	if email == "" {
		email, _ = claims["preferred_username"].(string)
	}
	if email == "" {
		return "", errors.New("azure id_token has no email claim")
	}
	return email, nil
}
