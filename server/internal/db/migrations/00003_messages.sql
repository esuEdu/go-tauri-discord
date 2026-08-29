-- +goose Up
-- +goose StatementBegin
CREATE TABLE messages (
    id         UUID PRIMARY KEY,
    channel_id UUID        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    author_id  UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at  TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX messages_channel_id_desc_idx
    ON messages (channel_id, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE attachments (
    id           UUID PRIMARY KEY,
    message_id   UUID   NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    storage_key  TEXT   NOT NULL,
    filename     TEXT   NOT NULL,
    size_bytes   BIGINT NOT NULL,
    content_type TEXT   NOT NULL
);

CREATE INDEX attachments_message_idx ON attachments (message_id);

CREATE TABLE read_states (
    user_id              UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel_id           UUID        NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    last_read_message_id UUID,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, channel_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE read_states;
DROP TABLE attachments;
DROP TABLE messages;
-- +goose StatementEnd
