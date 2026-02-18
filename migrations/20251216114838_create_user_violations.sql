-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_violations (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    violation_type VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_violations_chat_user_created ON user_violations(chat_id, user_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_violations;
-- +goose StatementEnd
