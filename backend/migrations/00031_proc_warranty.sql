-- +goose Up
-- +goose StatementBegin

-- Warranty term (in months) per purchase-order line. On receiving, each spawned
-- asset gets purchase_date = receipt date and warranty_expiry = receipt + term.
ALTER TABLE proc.po_lines ADD COLUMN warranty_months int NOT NULL DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE proc.po_lines DROP COLUMN IF EXISTS warranty_months;
-- +goose StatementEnd
