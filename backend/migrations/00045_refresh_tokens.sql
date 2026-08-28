-- +goose Up
-- +goose StatementBegin

-- Server-side refresh tokens for local (password) auth, replacing GoTrue's
-- auth.refresh_tokens. Stored hashed (sha256 hex) so a DB leak alone doesn't
-- hand out valid tokens. Rotated on every use: refresh marks the old row
-- revoked and inserts a new one.
CREATE TABLE iam.refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX refresh_tokens_hash_idx ON iam.refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_idx ON iam.refresh_tokens (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.refresh_tokens;
-- +goose StatementEnd
