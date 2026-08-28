-- +goose Up
-- +goose StatementBegin

-- Local auth logs in by email, and bootstrap needs ON CONFLICT (email) to be
-- idempotent - both require email to actually be unique.
CREATE UNIQUE INDEX user_profiles_email_idx ON iam.user_profiles (email);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS iam.user_profiles_email_idx;
-- +goose StatementEnd
