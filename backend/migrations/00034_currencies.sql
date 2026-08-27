-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Currencies: metadata-driven list used by every price/amount input. Amounts
-- are stored in their original currency; one currency is flagged as the default
-- (the reporting currency). Disable a currency to hide it from pickers without
-- losing historical references.
-- ---------------------------------------------------------------------------
CREATE TABLE meta.currencies (
    code       text PRIMARY KEY,            -- ISO 4217, e.g. USD, EUR, KES
    name       text NOT NULL,
    symbol     text NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    enabled    boolean NOT NULL DEFAULT true,
    sort       int NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Only one default currency at a time.
CREATE UNIQUE INDEX currencies_one_default ON meta.currencies (is_default) WHERE is_default;

INSERT INTO meta.currencies (code, name, symbol, is_default, sort) VALUES
    ('USD', 'US Dollar',         '$',  true,  1),
    ('EUR', 'Euro',              '€',  false, 2),
    ('GBP', 'British Pound',     '£',  false, 3),
    ('KES', 'Kenyan Shilling',   'KSh',false, 4),
    ('ZAR', 'South African Rand','R',  false, 5),
    ('NGN', 'Nigerian Naira',    '₦',  false, 6),
    ('AED', 'UAE Dirham',        'د.إ',false, 7),
    ('INR', 'Indian Rupee',      '₹',  false, 8),
    ('JPY', 'Japanese Yen',      '¥',  false, 9),
    ('CNY', 'Chinese Yuan',      '¥',  false, 10),
    ('CAD', 'Canadian Dollar',   '$',  false, 11),
    ('AUD', 'Australian Dollar', '$',  false, 12),
    ('CHF', 'Swiss Franc',       'Fr', false, 13)
ON CONFLICT (code) DO NOTHING;

DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'itam_app') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON meta.currencies TO itam_app;
  END IF;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS meta.currencies;
-- +goose StatementEnd
