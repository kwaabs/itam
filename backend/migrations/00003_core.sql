-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- core: the stable spine. Real columns + a jsonb attributes bag per asset.
-- ---------------------------------------------------------------------------

-- Location hierarchy: Region -> Site (DC/office) -> Room -> Rack -> Slot.
CREATE TABLE core.locations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key        citext NOT NULL UNIQUE,        -- ltree-safe slug
    name       text   NOT NULL,
    kind       text   NOT NULL DEFAULT 'site',
    parent_id  uuid REFERENCES core.locations(id) ON DELETE RESTRICT,
    path       ltree  NOT NULL,
    attributes jsonb  NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX locations_path_gist ON core.locations USING gist (path);
CREATE INDEX locations_parent_idx ON core.locations (parent_id);

-- Department / org-unit hierarchy.
CREATE TABLE core.org_units (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key        citext NOT NULL UNIQUE,
    name       text   NOT NULL,
    parent_id  uuid REFERENCES core.org_units(id) ON DELETE RESTRICT,
    path       ltree  NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX org_units_path_gist ON core.org_units USING gist (path);
CREATE INDEX org_units_parent_idx ON core.org_units (parent_id);

-- People (employees). manager_id models the supervision tree.
CREATE TABLE core.people (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_no citext UNIQUE,
    first_name  text NOT NULL,
    last_name   text NOT NULL,
    email       citext UNIQUE,
    title       text,
    org_unit_id uuid REFERENCES core.org_units(id) ON DELETE SET NULL,
    manager_id  uuid REFERENCES core.people(id) ON DELETE SET NULL,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX people_org_unit_idx ON core.people (org_unit_id);
CREATE INDEX people_manager_idx ON core.people (manager_id);

-- Assets: the heart of the system.
CREATE TABLE core.assets (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_tag          citext NOT NULL UNIQUE,
    serial             text,
    name               text   NOT NULL,
    asset_type_id      bigint NOT NULL REFERENCES meta.asset_types(id) ON DELETE RESTRICT,
    lifecycle_id       bigint REFERENCES meta.lifecycles(id) ON DELETE SET NULL,
    current_state_id   bigint REFERENCES meta.lifecycle_states(id) ON DELETE SET NULL,
    location_id        uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    owner_org_unit_id  uuid REFERENCES core.org_units(id) ON DELETE SET NULL,
    assigned_person_id uuid REFERENCES core.people(id) ON DELETE SET NULL,
    -- type-specific user-defined fields, validated at the app layer against meta.field_definitions
    attributes         jsonb  NOT NULL DEFAULT '{}',
    -- light financial fields (full procurement comes in a later phase)
    purchase_cost      numeric(14,2),
    purchase_date      date,
    warranty_expiry    date,
    vendor             text,
    notes              text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);
CREATE INDEX assets_type_idx       ON core.assets (asset_type_id);
CREATE INDEX assets_state_idx      ON core.assets (current_state_id);
CREATE INDEX assets_location_idx   ON core.assets (location_id);
CREATE INDEX assets_owner_idx      ON core.assets (owner_org_unit_id);
CREATE INDEX assets_assignee_idx   ON core.assets (assigned_person_id);
CREATE INDEX assets_attributes_gin ON core.assets USING gin (attributes);
CREATE INDEX assets_name_trgm      ON core.assets USING gin (name gin_trgm_ops);
CREATE INDEX assets_tag_trgm       ON core.assets USING gin ((asset_tag::text) gin_trgm_ops);

-- Custody history: who/what held an asset and movements between locations.
CREATE TABLE core.assignments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id         uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    kind             text NOT NULL DEFAULT 'assign',  -- assign|return|transfer
    holder_person_id uuid REFERENCES core.people(id) ON DELETE SET NULL,
    holder_org_unit_id uuid REFERENCES core.org_units(id) ON DELETE SET NULL,
    from_location_id uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    to_location_id   uuid REFERENCES core.locations(id) ON DELETE SET NULL,
    assigned_at      timestamptz NOT NULL DEFAULT now(),
    returned_at      timestamptz,
    assigned_by      uuid,                            -- GoTrue user id
    reason           text,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assignments_asset_idx ON core.assignments (asset_id);
CREATE INDEX assignments_open_idx  ON core.assignments (asset_id) WHERE returned_at IS NULL;

-- Typed relationships form the CMDB graph (rack-contains-server, server-runs-vm, ...).
CREATE TABLE core.asset_relationships (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    from_asset_id        uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    to_asset_id          uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    relationship_type_id bigint NOT NULL REFERENCES meta.relationship_types(id) ON DELETE RESTRICT,
    valid_from           timestamptz NOT NULL DEFAULT now(),
    valid_to             timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    CHECK (from_asset_id <> to_asset_id)
);
CREATE INDEX rel_from_idx ON core.asset_relationships (from_asset_id);
CREATE INDEX rel_to_idx   ON core.asset_relationships (to_asset_id);

-- Lifecycle transition history per asset.
CREATE TABLE core.lifecycle_history (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id      uuid NOT NULL REFERENCES core.assets(id) ON DELETE CASCADE,
    from_state_id bigint REFERENCES meta.lifecycle_states(id) ON DELETE SET NULL,
    to_state_id   bigint NOT NULL REFERENCES meta.lifecycle_states(id) ON DELETE RESTRICT,
    transition_id bigint REFERENCES meta.lifecycle_transitions(id) ON DELETE SET NULL,
    actor         uuid,
    note          text,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lifecycle_history_asset_idx ON core.lifecycle_history (asset_id);

-- Attachments live in S3 (RustFS); we only keep references.
CREATE TABLE core.attachments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type  text NOT NULL,                       -- asset|location|person|...
    entity_id    uuid NOT NULL,
    bucket       text NOT NULL,
    object_key   text NOT NULL,
    filename     text NOT NULL,
    content_type text,
    size_bytes   bigint,
    uploaded_by  uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX attachments_entity_idx ON core.attachments (entity_type, entity_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.attachments;
DROP TABLE IF EXISTS core.lifecycle_history;
DROP TABLE IF EXISTS core.asset_relationships;
DROP TABLE IF EXISTS core.assignments;
DROP TABLE IF EXISTS core.assets;
DROP TABLE IF EXISTS core.people;
DROP TABLE IF EXISTS core.org_units;
DROP TABLE IF EXISTS core.locations;
-- +goose StatementEnd
