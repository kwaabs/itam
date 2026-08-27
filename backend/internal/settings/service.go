package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
)

// Service reads the DB-backed, optionally Vault-encrypted settings table and
// caches resolved values in Valkey. Any failure resolves to (zero, false) so a
// missing table or misconfigured Vault never takes the app down -- callers fall
// back to environment configuration.
type Service struct {
	db  *bun.DB
	rdb *redis.Client
	ttl time.Duration
}

func New(db *bun.DB, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb, ttl: 60 * time.Second}
}

// Setting is one configuration row. Secret values are never serialised to JSON.
type Setting struct {
	Key         string          `bun:"key" json:"key"`
	Value       json.RawMessage `bun:"value" json:"value"`
	SecretName  string          `bun:"secret_name" json:"-"`
	IsSecret    bool            `bun:"is_secret" json:"is_secret"`
	Scope       string          `bun:"scope" json:"scope"`
	Description string          `bun:"description" json:"description"`
}

func cacheKey(k string) string { return "setting:" + strings.ToLower(k) }

// Get resolves a setting to a string, decrypting via Vault when it is a secret.
func (s *Service) Get(ctx context.Context, key string) (string, bool) {
	if s.rdb != nil {
		if v, err := s.rdb.Get(ctx, cacheKey(key)).Result(); err == nil {
			return v, true
		}
	}
	var row Setting
	err := s.db.NewRaw(
		`SELECT key, value, secret_name, is_secret, scope, description
		 FROM meta.settings WHERE key = ?`, key).Scan(ctx, &row)
	if err != nil {
		return "", false
	}
	val, ok := s.resolve(ctx, row)
	if !ok {
		return "", false
	}
	if s.rdb != nil {
		_ = s.rdb.Set(ctx, cacheKey(key), val, s.ttl).Err()
	}
	return val, true
}

// GetOr returns the resolved setting or the fallback when missing/empty.
func (s *Service) GetOr(ctx context.Context, key, fallback string) string {
	if v, ok := s.Get(ctx, key); ok && v != "" {
		return v
	}
	return fallback
}

func (s *Service) resolve(ctx context.Context, row Setting) (string, bool) {
	if row.IsSecret {
		if row.SecretName == "" {
			return "", false
		}
		var secret string
		if err := s.db.NewRaw(
			`SELECT meta.decrypt_secret(?)`,
			row.SecretName).Scan(ctx, &secret); err != nil {
			return "", false
		}
		return secret, true
	}
	return jsonToString(row.Value), true
}

func jsonToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return strings.TrimSpace(string(raw))
}

// SetSecretByKey rotates the Vault-encrypted secret behind a secret setting
// (e.g. azure_client_secret). The setting row must already exist and be marked
// is_secret; the plaintext never touches the value column. Cache is cleared so
// the new secret is picked up on the next read.
func (s *Service) SetSecretByKey(ctx context.Context, key, plaintext string) error {
	var secretName string
	if err := s.db.NewRaw(
		`SELECT secret_name FROM meta.settings WHERE key = ? AND is_secret = true`,
		key).Scan(ctx, &secretName); err != nil {
		return errors.New("unknown secret setting")
	}
	if secretName == "" {
		return errors.New("not a secret setting")
	}
	if _, err := s.db.NewRaw(`SELECT meta.set_secret(?, ?)`, secretName, plaintext).Exec(ctx); err != nil {
		return err
	}
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, cacheKey(key)).Err()
	}
	return nil
}

// ListPublic returns the non-secret settings for the admin API.
func (s *Service) ListPublic(ctx context.Context) ([]Setting, error) {
	var rows []Setting
	err := s.db.NewRaw(
		`SELECT key, value, secret_name, is_secret, scope, description
		 FROM meta.settings WHERE is_secret = false ORDER BY scope, key`).Scan(ctx, &rows)
	return rows, err
}

// SetPublic upserts a non-secret setting value and clears its cache. Secret
// rows are protected by the WHERE clause (they cannot be edited via this path).
func (s *Service) SetPublic(ctx context.Context, key string, value json.RawMessage) error {
	_, err := s.db.NewRaw(
		`INSERT INTO meta.settings (key, value, is_secret, scope, updated_at)
		 VALUES (?, ?, false, 'app', now())
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
		 WHERE meta.settings.is_secret = false`, key, value).Exec(ctx)
	if err == nil && s.rdb != nil {
		_ = s.rdb.Del(ctx, cacheKey(key)).Err()
	}
	return err
}
