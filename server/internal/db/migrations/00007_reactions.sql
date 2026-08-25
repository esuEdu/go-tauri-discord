-- +goose Up
-- +goose StatementBegin
CREATE TABLE message_reactions (
    message_id UUID        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    emoji      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, emoji)
);

UPDATE roles SET permissions = permissions | 8192 WHERE permissions & 2 <> 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE roles SET permissions = permissions & ~8192::BIGINT;

DROP TABLE message_reactions;
-- +goose StatementEnd
