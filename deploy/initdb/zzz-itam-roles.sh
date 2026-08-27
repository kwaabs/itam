#!/bin/bash
# Mounted as a single file into /docker-entrypoint-initdb.d so it runs AFTER the
# supabase/postgres image's own init scripts (the zzz- prefix sorts last) without
# shadowing them. Runs once, on a fresh data dir only -- use `make infra-clean`
# to re-run. Creates the least-privilege login roles with passwords from env;
# table-level grants live in the goose migrations.
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	-- API connects with this least-privilege role (grants come from migrations).
	DO \$do\$
	BEGIN
	  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
	    ALTER ROLE itam_app WITH LOGIN PASSWORD '${ITAM_DB_PASSWORD}';
	  ELSE
	    CREATE ROLE itam_app WITH LOGIN PASSWORD '${ITAM_DB_PASSWORD}';
	  END IF;
	END
	\$do\$;

	-- GoTrue connects as supabase_auth_admin (created by the supabase image).
	DO \$do\$
	BEGIN
	  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN
	    ALTER ROLE supabase_auth_admin WITH LOGIN PASSWORD '${AUTH_DB_PASSWORD}';
	  ELSE
	    CREATE ROLE supabase_auth_admin WITH LOGIN CREATEROLE PASSWORD '${AUTH_DB_PASSWORD}';
	  END IF;
	END
	\$do\$;

	-- Pre-create the extensions GoTrue expects so its role never needs superuser.
	CREATE EXTENSION IF NOT EXISTS pgcrypto;
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	-- GoTrue owns its own schema.
	CREATE SCHEMA IF NOT EXISTS auth AUTHORIZATION supabase_auth_admin;
	ALTER SCHEMA auth OWNER TO supabase_auth_admin;
	GRANT ALL ON SCHEMA auth TO supabase_auth_admin;
EOSQL
