-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- iam: hierarchical RBAC layered on top of GoTrue.
-- Authentication (passwords, OIDC/Azure, JWT issuance) is owned by GoTrue in
-- its own `auth` schema. We only keep app-level identity + authorization here,
-- loosely coupled by the GoTrue user id (uuid). No hard FK to auth.users so the
-- two services can migrate independently.
-- ---------------------------------------------------------------------------

-- App-level profile mapping a GoTrue user to a person + flags.
CREATE TABLE iam.user_profiles (
    user_id      uuid PRIMARY KEY,                    -- GoTrue auth.users.id
    email        citext,
    display_name text,
    person_id    uuid REFERENCES core.people(id) ON DELETE SET NULL,
    is_superuser boolean NOT NULL DEFAULT false,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE iam.permissions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key         citext NOT NULL UNIQUE,               -- e.g. asset.read, asset.transition
    description text
);

CREATE TABLE iam.roles (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE iam.role_permissions (
    role_id       bigint NOT NULL REFERENCES iam.roles(id) ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES iam.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- A grant gives a user a role within a hierarchical scope.
--   scope_type = global       -> applies everywhere
--   scope_type = location     -> applies to scope_path subtree of core.locations
--   scope_type = org_unit     -> applies to scope_path subtree of core.org_units
CREATE TABLE iam.role_grants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid   NOT NULL,
    role_id    bigint NOT NULL REFERENCES iam.roles(id) ON DELETE CASCADE,
    scope_type text   NOT NULL DEFAULT 'global',
    scope_path ltree,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (scope_type IN ('global','location','org_unit')),
    CHECK (scope_type = 'global' OR scope_path IS NOT NULL)
);
CREATE INDEX role_grants_user_idx  ON iam.role_grants (user_id);
CREATE INDEX role_grants_path_gist ON iam.role_grants USING gist (scope_path);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.role_grants;
DROP TABLE IF EXISTS iam.role_permissions;
DROP TABLE IF EXISTS iam.roles;
DROP TABLE IF EXISTS iam.permissions;
DROP TABLE IF EXISTS iam.user_profiles;
-- +goose StatementEnd
