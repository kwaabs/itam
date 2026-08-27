-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Native Azure AD (Entra ID) OIDC login, fully metadata-driven so tenant /
-- client id / secret can change at runtime without a redeploy. Reuses the
-- azure_* settings from 00008 and adds the tenant + redirect URL. A SECURITY
-- DEFINER accessor lets the app role rotate the Vault-encrypted client secret.
-- ---------------------------------------------------------------------------

INSERT INTO meta.settings (key, value, is_secret, scope, description) VALUES
    ('azure_tenant',       '""'::jsonb, false, 'auth', 'Azure AD directory (tenant) ID'),
    ('azure_redirect_url', '""'::jsonb, false, 'auth', 'OAuth redirect URL (blank = API /auth/azure/callback)')
ON CONFLICT (key) DO NOTHING;

-- Write/rotate a Vault secret by name. Mirrors meta.decrypt_secret (00009): the
-- app role only gets EXECUTE, the decryption/encryption runs as the owner. The
-- vault.* calls go through EXECUTE so the exact create/update_secret signature
-- (which differs across supabase/postgres builds) is resolved at run time rather
-- than at CREATE FUNCTION time under check_function_bodies.
CREATE OR REPLACE FUNCTION meta.set_secret(p_name text, p_secret text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = vault, public
AS $$
DECLARE sid uuid;
BEGIN
  EXECUTE 'SELECT id FROM vault.secrets WHERE name = $1' INTO sid USING p_name;
  IF sid IS NULL THEN
    EXECUTE 'SELECT vault.create_secret($1, $2)' USING p_secret, p_name;
  ELSE
    BEGIN
      EXECUTE 'SELECT vault.update_secret($1, $2)' USING sid, p_secret;
    EXCEPTION WHEN undefined_function THEN
      -- Older Vault builds lack update_secret: replace the row instead.
      EXECUTE 'DELETE FROM vault.secrets WHERE id = $1' USING sid;
      EXECUTE 'SELECT vault.create_secret($1, $2)' USING p_secret, p_name;
    END;
  END IF;
END;
$$;

REVOKE ALL ON FUNCTION meta.set_secret(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta.set_secret(text, text) TO itam_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS meta.set_secret(text, text);
DELETE FROM meta.settings WHERE key IN ('azure_tenant', 'azure_redirect_url');
-- +goose StatementEnd
