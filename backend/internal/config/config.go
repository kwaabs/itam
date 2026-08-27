package config

import (
	"os"
	"strings"
)

// Config holds all runtime configuration, sourced from the environment so the
// same binary runs in dev and prod (12-factor style).
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
	NATSURL     string

	// GoTrueJWTSecret signs/verifies our own locally-issued tokens (name kept
	// for now - GoTrue itself is still running until it's fully retired).
	GoTrueJWTSecret string

	// Object storage (RustFS, S3-compatible) for attachments.
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string

	// Bootstrap admin. The matching GoTrue user is granted the admin role and
	// flagged superuser on first login (just-in-time provisioning).
	AdminEmail    string
	AdminPassword string

	CORSOrigins []string
	Debug       bool
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// Load reads configuration from the environment, applying sensible dev defaults.
func Load() Config {
	// Local-dev defaults target the 5600-5620 host port range used by the
	// docker infra stack, so `make run` / `make web` work against `make infra-up`.
	return Config{
		HTTPAddr:    env("HTTP_ADDR", ":5607"),
		DatabaseURL: env("DATABASE_URL", "postgres://itam_app:itam_app_dev_pw@localhost:5600/postgres?sslmode=disable"),
		RedisAddr:   env("REDIS_ADDR", "localhost:5601"),
		RedisPass:   env("REDIS_PASSWORD", ""),
		NATSURL:     env("NATS_URL", "nats://localhost:5602"),

		GoTrueJWTSecret: env("GOTRUE_JWT_SECRET", "super-secret-jwt-token-with-at-least-32-characters-long"),

		S3Endpoint:  env("S3_ENDPOINT", "http://localhost:5604"),
		S3Region:    env("S3_REGION", "us-east-1"),
		S3Bucket:    env("S3_BUCKET", "itam"),
		S3AccessKey: env("S3_ACCESS_KEY", "rustfsadmin"),
		S3SecretKey: env("S3_SECRET_KEY", "rustfsadmin"),

		AdminEmail:    env("ADMIN_EMAIL", "admin@itam.local"),
		AdminPassword: env("ADMIN_PASSWORD", "admin12345"),

		// 5608 = SvelteKit web; 5609 = Flutter web (field app) dev server.
		CORSOrigins: strings.Split(env("CORS_ORIGINS", "http://localhost:5608,http://localhost:5609"), ","),
		Debug:       env("DEBUG", "true") == "true",
	}
}
