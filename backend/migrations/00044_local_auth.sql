-- +goose Up
-- +goose StatementBegin

-- Local auth: the app now owns credentials directly instead of delegating to
-- GoTrue. password_hash is bcrypt (golang.org/x/crypto/bcrypt); NULL means the
-- account has no local password (Azure-only, or not yet set).
ALTER TABLE iam.user_profiles ADD COLUMN password_hash text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE iam.user_profiles DROP COLUMN password_hash;
-- +goose StatementEnd
