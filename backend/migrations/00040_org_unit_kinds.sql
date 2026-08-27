-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Org hierarchy kinds: region → district → division → unit (configurable).
-- Assets link via owner_org_unit_id; subtree rollups power grouping reports.
-- ---------------------------------------------------------------------------
CREATE TABLE meta.org_unit_kinds (
    key   citext PRIMARY KEY,
    label text   NOT NULL,
    sort  int    NOT NULL DEFAULT 0
);

INSERT INTO meta.org_unit_kinds (key, label, sort) VALUES
    ('region',   'Region',   10),
    ('district', 'District', 20),
    ('division', 'Division', 30),
    ('unit',     'Unit',     40);

GRANT SELECT, INSERT, UPDATE, DELETE ON meta.org_unit_kinds TO itam_app;

ALTER TABLE core.org_units
    ADD COLUMN kind citext REFERENCES meta.org_unit_kinds(key) ON DELETE RESTRICT;

CREATE INDEX org_units_kind_idx ON core.org_units (kind);

-- Root org is a region by default.
UPDATE core.org_units SET kind = 'region' WHERE key = 'org' AND kind IS NULL;

-- Optional demo hierarchy (safe to re-run: keys are unique).
INSERT INTO core.org_units (key, name, parent_id, path, kind)
SELECT 'north', 'North Region', p.id, 'org.north'::ltree, 'region'
FROM core.org_units p WHERE p.key = 'org'
ON CONFLICT (key) DO UPDATE SET kind = EXCLUDED.kind, name = EXCLUDED.name;

INSERT INTO core.org_units (key, name, parent_id, path, kind)
SELECT 'north_central', 'North Central District', p.id, 'org.north.north_central'::ltree, 'district'
FROM core.org_units p WHERE p.key = 'north'
ON CONFLICT (key) DO UPDATE SET kind = EXCLUDED.kind, name = EXCLUDED.name;

INSERT INTO core.org_units (key, name, parent_id, path, kind)
SELECT 'it_ops', 'IT Operations', p.id, 'org.north.north_central.it_ops'::ltree, 'division'
FROM core.org_units p WHERE p.key = 'north_central'
ON CONFLICT (key) DO UPDATE SET kind = EXCLUDED.kind, name = EXCLUDED.name;

INSERT INTO core.org_units (key, name, parent_id, path, kind)
SELECT 'helpdesk', 'Helpdesk Unit', p.id, 'org.north.north_central.it_ops.helpdesk'::ltree, 'unit'
FROM core.org_units p WHERE p.key = 'it_ops'
ON CONFLICT (key) DO UPDATE SET kind = EXCLUDED.kind, name = EXCLUDED.name;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.org_units WHERE key IN ('helpdesk', 'it_ops', 'north_central', 'north');
ALTER TABLE core.org_units DROP COLUMN IF EXISTS kind;
DROP TABLE IF EXISTS meta.org_unit_kinds;
-- +goose StatementEnd
