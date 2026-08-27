-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- meta: everything that varies between orgs lives here as editable rows.
-- ---------------------------------------------------------------------------

-- Field data types drive validation + frontend rendering.
CREATE TABLE meta.data_types (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key         citext NOT NULL UNIQUE,
    label       text   NOT NULL,
    -- value_kind tells the engine how to validate/store: text|number|bool|date|datetime|json|enum|reference
    value_kind  text   NOT NULL DEFAULT 'text',
    description text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta.units (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key     citext NOT NULL UNIQUE,
    label   text   NOT NULL,
    symbol  text
);

CREATE TABLE meta.categories (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key       citext NOT NULL UNIQUE,
    label     text   NOT NULL,
    parent_id bigint REFERENCES meta.categories(id) ON DELETE SET NULL
);

CREATE TABLE meta.manufacturers (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key  citext NOT NULL UNIQUE,
    name text   NOT NULL
);

CREATE TABLE meta.models (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key             citext NOT NULL UNIQUE,
    name            text   NOT NULL,
    manufacturer_id bigint REFERENCES meta.manufacturers(id) ON DELETE SET NULL,
    category_id     bigint REFERENCES meta.categories(id) ON DELETE SET NULL
);

-- Lifecycle = a named state machine that asset types can bind to.
CREATE TABLE meta.lifecycles (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key         citext NOT NULL UNIQUE,
    name        text   NOT NULL,
    description text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta.lifecycle_states (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lifecycle_id bigint NOT NULL REFERENCES meta.lifecycles(id) ON DELETE CASCADE,
    key          citext NOT NULL,
    label        text   NOT NULL,
    is_initial   boolean NOT NULL DEFAULT false,
    is_terminal  boolean NOT NULL DEFAULT false,
    color        text,
    sort         int    NOT NULL DEFAULT 0,
    UNIQUE (lifecycle_id, key)
);

CREATE TABLE meta.lifecycle_transitions (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lifecycle_id        bigint NOT NULL REFERENCES meta.lifecycles(id) ON DELETE CASCADE,
    key                 citext NOT NULL,
    label               text   NOT NULL,
    -- from_state_id NULL means "from any state" (or the initial creation transition)
    from_state_id       bigint REFERENCES meta.lifecycle_states(id) ON DELETE CASCADE,
    to_state_id         bigint NOT NULL REFERENCES meta.lifecycle_states(id) ON DELETE CASCADE,
    required_permission text,                         -- e.g. asset.transition
    required_fields     jsonb  NOT NULL DEFAULT '[]', -- array of field keys that must be present
    guard               jsonb  NOT NULL DEFAULT '{}', -- optional predicate on the asset
    emit_subject        text   NOT NULL DEFAULT 'itam.asset.state_changed',
    sort                int    NOT NULL DEFAULT 0,
    UNIQUE (lifecycle_id, key)
);

-- Asset types form a hierarchy: Hardware -> Computer -> Laptop, etc.
CREATE TABLE meta.asset_types (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key          citext NOT NULL UNIQUE,
    name         text   NOT NULL,
    parent_id    bigint REFERENCES meta.asset_types(id) ON DELETE SET NULL,
    path         ltree  NOT NULL,
    lifecycle_id bigint REFERENCES meta.lifecycles(id) ON DELETE SET NULL,
    icon         text,
    is_abstract  boolean NOT NULL DEFAULT false,  -- abstract types cannot have asset instances
    sort         int    NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX asset_types_path_gist ON meta.asset_types USING gist (path);

-- Field definitions describe the type-specific attributes stored in assets.attributes (jsonb).
CREATE TABLE meta.field_definitions (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    asset_type_id    bigint NOT NULL REFERENCES meta.asset_types(id) ON DELETE CASCADE,
    key              citext NOT NULL,
    label            text   NOT NULL,
    data_type_id     bigint NOT NULL REFERENCES meta.data_types(id),
    required         boolean NOT NULL DEFAULT false,
    is_unique        boolean NOT NULL DEFAULT false,
    default_value    jsonb,
    validation       jsonb  NOT NULL DEFAULT '{}',   -- {min,max,regex,minLength,maxLength}
    unit_id          bigint REFERENCES meta.units(id) ON DELETE SET NULL,
    enum_options     jsonb  NOT NULL DEFAULT '[]',   -- [{value,label}] for enum data types
    reference_target text,                            -- e.g. core.people, core.locations
    indexed          boolean NOT NULL DEFAULT false,
    help_text        text,
    sort             int    NOT NULL DEFAULT 0,
    UNIQUE (asset_type_id, key)
);

CREATE TABLE meta.relationship_types (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key           citext NOT NULL UNIQUE,
    label         text   NOT NULL,
    inverse_label text,
    -- optional asset_type keys constraining either end (NULL = any)
    from_type_key citext,
    to_type_key   citext,
    cardinality   text   NOT NULL DEFAULT 'many_to_many', -- one_to_one|one_to_many|many_to_many
    sort          int    NOT NULL DEFAULT 0
);

-- Configurable automation: react to events without code.
CREATE TABLE meta.automation_rules (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        citext NOT NULL UNIQUE,
    name       text   NOT NULL,
    on_subject text   NOT NULL,                 -- NATS subject pattern, e.g. itam.asset.state_changed
    condition  jsonb  NOT NULL DEFAULT '{}',    -- predicate evaluated against the event payload
    action     jsonb  NOT NULL DEFAULT '{}',    -- {type: "log"|"audit"|"webhook", ...}
    enabled    boolean NOT NULL DEFAULT true,
    sort       int    NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS meta.automation_rules;
DROP TABLE IF EXISTS meta.relationship_types;
DROP TABLE IF EXISTS meta.field_definitions;
DROP TABLE IF EXISTS meta.asset_types;
DROP TABLE IF EXISTS meta.lifecycle_transitions;
DROP TABLE IF EXISTS meta.lifecycle_states;
DROP TABLE IF EXISTS meta.lifecycles;
DROP TABLE IF EXISTS meta.models;
DROP TABLE IF EXISTS meta.manufacturers;
DROP TABLE IF EXISTS meta.categories;
DROP TABLE IF EXISTS meta.units;
DROP TABLE IF EXISTS meta.data_types;
-- +goose StatementEnd
