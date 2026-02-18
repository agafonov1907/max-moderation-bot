-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS temporary_messages (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    message_id TEXT NOT NULL,
    delete_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_temporary_messages_delete_at ON temporary_messages(delete_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS temporary_messages;
-- +goose StatementEnd
