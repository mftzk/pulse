-- +goose Up
-- +goose StatementBegin
-- email is nullable so pre-existing accounts (created before signup required an
-- email) keep working. New signups always supply one. UNIQUE permits many NULLs
-- in Postgres, so the constraint only bites real, duplicate addresses.
ALTER TABLE users ADD COLUMN email TEXT UNIQUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN email;
-- +goose StatementEnd
