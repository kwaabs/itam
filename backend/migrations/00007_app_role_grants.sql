-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Least-privilege grants for the application role (itam_app).
-- The role's LOGIN + password are set by the db init script (deploy/initdb) so
-- no secret lives in version control. Here we guard-create a NOLOGIN fallback
-- (so grants never fail if the init script didn't run) and wire privileges on
-- the app schemas only -- never the bootstrap superuser.
-- ---------------------------------------------------------------------------

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    CREATE ROLE itam_app NOLOGIN;
  END IF;
END
$$;

GRANT USAGE ON SCHEMA meta, core, iam, audit TO itam_app;

-- Existing objects.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA meta, core, iam, audit TO itam_app;
GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA meta, core, iam, audit TO itam_app;

-- Future objects created by the migration superuser in these schemas
-- (e.g. meta.settings in the next migration) are auto-granted.
ALTER DEFAULT PRIVILEGES IN SCHEMA meta, core, iam, audit
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA meta, core, iam, audit
  GRANT USAGE, SELECT ON SEQUENCES TO itam_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER DEFAULT PRIVILEGES IN SCHEMA meta, core, iam, audit
  REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM itam_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA meta, core, iam, audit
  REVOKE USAGE, SELECT ON SEQUENCES FROM itam_app;
REVOKE ALL ON ALL TABLES    IN SCHEMA meta, core, iam, audit FROM itam_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA meta, core, iam, audit FROM itam_app;
REVOKE USAGE ON SCHEMA meta, core, iam, audit FROM itam_app;
-- The role itself is left intact (it may own session state); drop manually.
-- +goose StatementEnd
