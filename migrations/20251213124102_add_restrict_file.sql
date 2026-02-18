-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_settings ADD COLUMN IF NOT EXISTS restrict_file BOOLEAN DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chat_settings DROP COLUMN IF EXISTS restrict_file;
-- +goose StatementEnd
