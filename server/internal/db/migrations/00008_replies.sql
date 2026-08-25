-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages
    ADD COLUMN reply_to_message_id UUID REFERENCES messages (id) ON DELETE SET NULL;

CREATE INDEX messages_reply_to_idx
    ON messages (reply_to_message_id)
    WHERE reply_to_message_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX messages_reply_to_idx;

ALTER TABLE messages DROP COLUMN reply_to_message_id;
-- +goose StatementEnd
