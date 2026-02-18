-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chat_stats (
    chat_id BIGINT NOT NULL,
    date DATE NOT NULL,
    word_violations BIGINT DEFAULT 0,
    link_violations BIGINT DEFAULT 0,
    image_violations BIGINT DEFAULT 0,
    video_violations BIGINT DEFAULT 0,
    audio_violations BIGINT DEFAULT 0,
    file_violations BIGINT DEFAULT 0,
    mute_count BIGINT DEFAULT 0,
    PRIMARY KEY (chat_id, date)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chat_stats;
-- +goose StatementEnd
