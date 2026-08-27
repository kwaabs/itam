-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Flexible, DB-backed settings. Non-secret values live inline as jsonb; secret
-- values are encrypted at rest with Supabase Vault (pgsodium) and referenced by
-- name. The API loads these at runtime (cached in Valkey) and falls back to env,
-- so config can change without a redeploy and secrets stay out of plaintext.
-- ---------------------------------------------------------------------------

CREATE EXTENSION IF NOT EXISTS supabase_vault CASCADE;

CREATE TABLE meta.settings (
    key         citext PRIMARY KEY,
    value       jsonb,                 -- non-secret value
    secret_name text,                  -- vault secret name (secret values)
    is_secret   boolean NOT NULL DEFAULT false,
    scope       text NOT NULL DEFAULT 'app',
    description text,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (is_secret = (secret_name IS NOT NULL))
);

-- Non-secret defaults (overridable at runtime).
INSERT INTO meta.settings (key, value, is_secret, scope, description) VALUES
    ('s3_endpoint',     '"http://rustfs:9000"'::jsonb, false, 'storage', 'S3 endpoint'),
    ('s3_region',       '"us-east-1"'::jsonb,          false, 'storage', 'S3 region'),
    ('s3_bucket',       '"itam"'::jsonb,               false, 'storage', 'S3 bucket'),
    ('azure_enabled',   'false'::jsonb,                false, 'auth',    'Azure AD SSO enabled'),
    ('azure_client_id', '""'::jsonb,                   false, 'auth',    'Azure AD client id'),
    ('azure_url',       '""'::jsonb,                   false, 'auth',    'Azure AD OIDC issuer URL');

-- Secret defaults, encrypted via Vault. vault.create_secret(secret, name, desc).
SELECT vault.create_secret('rustfsadmin', 's3_access_key',       'S3 access key');
SELECT vault.create_secret('rustfsadmin', 's3_secret_key',       'S3 secret key');
SELECT vault.create_secret('',            'azure_client_secret', 'Azure AD client secret');

INSERT INTO meta.settings (key, secret_name, is_secret, scope, description) VALUES
    ('s3_access_key',       's3_access_key',       true, 'storage', 'S3 access key (Vault)'),
    ('s3_secret_key',       's3_secret_key',       true, 'storage', 'S3 secret key (Vault)'),
    ('azure_client_secret', 'azure_client_secret', true, 'auth',    'Azure AD client secret (Vault)');

-- Let the app role read settings and decrypt its own secrets. The
-- vault.decrypted_secrets view runs with its (superuser) owner's privileges, so
-- SELECT on the view is enough to read decrypted values. pgsodium_keyholder is
-- granted only when present (it doesn't exist in all supabase/postgres builds).
GRANT SELECT, INSERT, UPDATE, DELETE ON meta.settings TO itam_app;
GRANT USAGE ON SCHEMA vault TO itam_app;
GRANT SELECT ON vault.decrypted_secrets TO itam_app;
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgsodium_keyholder') THEN
    GRANT pgsodium_keyholder TO itam_app;
  END IF;
END
$$;

-- Settings management permissions (admin already gets everything).
INSERT INTO iam.permissions (key, description) VALUES
    ('settings.read',   'View application settings (non-secret)'),
    ('settings.manage', 'Edit application settings');

INSERT INTO iam.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM iam.roles r CROSS JOIN iam.permissions p
WHERE r.key = 'admin' AND p.key IN ('settings.read', 'settings.manage');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM iam.permissions WHERE key IN ('settings.read', 'settings.manage');
DROP TABLE IF EXISTS meta.settings;
-- Vault secrets and the extension are intentionally left in place.
-- +goose StatementEnd
