-- +goose Up
-- +goose StatementBegin

-- vault.decrypted_secrets runs with security_invoker in this supabase/postgres
-- build, so a least-privilege role can't decrypt through it directly (the
-- pgsodium_keyholder role doesn't exist to grant). Expose a narrow SECURITY
-- DEFINER accessor owned by the superuser instead: it decrypts as the owner and
-- only returns a single named secret. The app role gets EXECUTE, nothing more.
CREATE OR REPLACE FUNCTION meta.decrypt_secret(p_name text)
RETURNS text
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = vault, public
AS $$
  SELECT decrypted_secret FROM vault.decrypted_secrets WHERE name = p_name;
$$;

REVOKE ALL ON FUNCTION meta.decrypt_secret(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta.decrypt_secret(text) TO itam_app;

-- The direct view grant is no longer needed; the app uses the accessor.
REVOKE SELECT ON vault.decrypted_secrets FROM itam_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
GRANT SELECT ON vault.decrypted_secrets TO itam_app;
DROP FUNCTION IF EXISTS meta.decrypt_secret(text);
-- +goose StatementEnd
