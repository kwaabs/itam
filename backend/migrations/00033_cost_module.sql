-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Cost module: straight-line depreciation (useful life per asset type) plus a
-- lightweight budgeting layer. Cost centers are existing org units; budgets are
-- tracked per org unit per fiscal year, actuals roll up the org-unit subtree.
-- ---------------------------------------------------------------------------

-- Depreciation: useful life (months) per asset type. 0 = do not depreciate.
ALTER TABLE meta.asset_types ADD COLUMN IF NOT EXISTS useful_life_months int NOT NULL DEFAULT 0;

-- Sensible straight-line defaults for common types (only where present).
UPDATE meta.asset_types SET useful_life_months = 48 WHERE key IN ('laptop','desktop','workstation','tablet','phone','mobile') AND useful_life_months = 0;
UPDATE meta.asset_types SET useful_life_months = 60 WHERE key IN ('server','blade','storage','san','nas','switch','router','firewall','load_balancer','ups','pdu','monitor') AND useful_life_months = 0;
UPDATE meta.asset_types SET useful_life_months = 84 WHERE key IN ('generator','transformer','ats','sts','avr','chiller','crac','crah','inrow') AND useful_life_months = 0;

-- Budgets per org unit + fiscal year (single reporting currency assumed).
CREATE SCHEMA IF NOT EXISTS fin;

CREATE TABLE fin.budgets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_unit_id uuid NOT NULL REFERENCES core.org_units(id) ON DELETE CASCADE,
    period_year int  NOT NULL,
    amount      numeric(14,2) NOT NULL DEFAULT 0,
    currency    text NOT NULL DEFAULT 'USD',
    notes       text,
    created_by  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_unit_id, period_year)
);
CREATE INDEX budgets_org_unit_idx ON fin.budgets (org_unit_id);

-- --- Least-privilege grants for the app role on the new schema --------------
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT USAGE ON SCHEMA fin TO itam_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA fin TO itam_app;
    GRANT USAGE, SELECT                ON ALL SEQUENCES IN SCHEMA fin TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA fin GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO itam_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA fin GRANT USAGE, SELECT ON SEQUENCES TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS fin CASCADE;
ALTER TABLE meta.asset_types DROP COLUMN IF EXISTS useful_life_months;
-- +goose StatementEnd
