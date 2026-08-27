-- +goose Up
-- +goose StatementBegin

-- Extensions (provided by the Supabase postgres image, all OSS/FOSS).
CREATE EXTENSION IF NOT EXISTS pgcrypto;      -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- uuid helpers
CREATE EXTENSION IF NOT EXISTS ltree;         -- hierarchical paths (locations / org units / rbac scopes)
CREATE EXTENSION IF NOT EXISTS pg_trgm;       -- fuzzy / substring search on names & tags
CREATE EXTENSION IF NOT EXISTS citext;        -- case-insensitive keys, emails, asset tags
CREATE EXTENSION IF NOT EXISTS btree_gist;    -- composite GiST (range/temporal exclusion constraints)

-- Dedicated schemas keep things tidy instead of dumping everything into public.
CREATE SCHEMA IF NOT EXISTS meta;   -- runtime-editable metadata / configuration
CREATE SCHEMA IF NOT EXISTS core;   -- the stable data spine
CREATE SCHEMA IF NOT EXISTS iam;    -- users, roles, permissions, scoped grants
CREATE SCHEMA IF NOT EXISTS audit;  -- append-only audit log

COMMENT ON SCHEMA meta IS 'Metadata-first config: asset types, fields, lifecycles, relationship types, automation rules';
COMMENT ON SCHEMA core IS 'Stable spine: assets, hierarchies, assignments, relationships';
COMMENT ON SCHEMA iam IS 'Local auth + hierarchical RBAC';
COMMENT ON SCHEMA audit IS 'Append-only audit trail fed by the event bus';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS iam CASCADE;
DROP SCHEMA IF EXISTS core CASCADE;
DROP SCHEMA IF EXISTS meta CASCADE;
-- +goose StatementEnd
