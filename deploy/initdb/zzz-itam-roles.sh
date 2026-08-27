#!/bin/bash
# Mounted as a single file into /docker-entrypoint-initdb.d so it runs AFTER the
# supabase/postgres image's own init scripts (the zzz- prefix sorts last) without
# shadowing them. Runs once, on a fresh data dir only -- use `make infra-clean`
# to re-run. Creates the least-privilege login role with a password from env;
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
EOSQL
