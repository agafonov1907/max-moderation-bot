-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chat_settings (
    chat_id BIGINT PRIMARY KEY,
    blocked_words TEXT[],
    blocked_domains TEXT[],
    restrict_image BOOLEAN DEFAULT FALSE,
    restrict_video BOOLEAN DEFAULT FALSE,
    restrict_audio BOOLEAN DEFAULT FALSE,
    enable_word_filter BOOLEAN DEFAULT TRUE,
    enable_link_filter BOOLEAN DEFAULT TRUE,
    enable_mute BOOLEAN DEFAULT FALSE,
    enable_auto_delete BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_states (
    user_id BIGINT PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mutes (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT,
    user_id BIGINT,
    expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_mutes_chat_id ON mutes(chat_id);
CREATE INDEX IF NOT EXISTS idx_mutes_user_id ON mutes(user_id);
CREATE INDEX IF NOT EXISTS idx_mutes_expires_at ON mutes(expires_at);

CREATE TABLE IF NOT EXISTS link_tokens (
    token TEXT PRIMARY KEY,
    user_id BIGINT,
    expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_link_tokens_user_id ON link_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_link_tokens_expires_at ON link_tokens(expires_at);

CREATE TABLE IF NOT EXISTS chat_admins (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT,
    user_id BIGINT,
    created_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_user ON chat_admins(chat_id, user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_admins;
DROP TABLE IF EXISTS link_tokens;
DROP TABLE IF EXISTS mutes;
DROP TABLE IF EXISTS user_states;
DROP TABLE IF EXISTS chat_settings;
-- +goose StatementEnd
