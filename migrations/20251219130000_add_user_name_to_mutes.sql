-- +goose Up
-- +goose StatementBegin
ALTER TABLE mutes ADD COLUMN IF NOT EXISTS user_name VARCHAR(255);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE mutes DROP COLUMN IF EXISTS user_name;
-- +goose StatementEnd
