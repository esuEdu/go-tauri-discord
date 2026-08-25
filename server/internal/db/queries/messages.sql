-- name: CreateMessage :one
INSERT INTO messages (id, channel_id, author_id, content, reply_to_message_id)
VALUES (@id, @channel_id, @author_id, @content, sqlc.narg('reply_to_message_id'))
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = @id AND deleted_at IS NULL;

-- name: ListMessages :many
SELECT
    m.*,
    u.username   AS author_username,
    u.avatar_key AS author_avatar_key
FROM messages m
JOIN users u ON u.id = m.author_id
WHERE m.channel_id = @channel_id
  AND m.deleted_at IS NULL
  AND (sqlc.narg('before')::uuid IS NULL OR m.id < sqlc.narg('before')::uuid)
ORDER BY m.id DESC
LIMIT @page_size;

-- name: UpdateMessageContent :one
UPDATE messages
SET content = @content, edited_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMessage :exec
UPDATE messages SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: CreateAttachment :one
INSERT INTO attachments (id, message_id, storage_key, filename, size_bytes, content_type)
VALUES (@id, @message_id, @storage_key, @filename, @size_bytes, @content_type)
RETURNING *;

-- name: ListAttachmentsForMessages :many
SELECT * FROM attachments
WHERE message_id = ANY (@message_ids::uuid[])
ORDER BY message_id, id;

-- name: UpsertReadState :exec
INSERT INTO read_states (user_id, channel_id, last_read_message_id, updated_at)
VALUES (@user_id, @channel_id, @last_read_message_id, now())
ON CONFLICT (user_id, channel_id)
DO UPDATE SET
    last_read_message_id = EXCLUDED.last_read_message_id,
    updated_at           = now();

-- name: ListReadStates :many
SELECT * FROM read_states WHERE user_id = @user_id;

-- name: ListLatestMessageIDs :many
SELECT DISTINCT ON (channel_id) channel_id, id
FROM messages
WHERE channel_id = ANY (@channel_ids::uuid[])
  AND deleted_at IS NULL
ORDER BY channel_id, id DESC;

-- name: GetAttachment :one
SELECT * FROM attachments WHERE id = @id;

-- name: DeleteAttachmentsForMessage :many
DELETE FROM attachments WHERE message_id = @message_id RETURNING *;
