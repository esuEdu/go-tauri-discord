-- A deleted parent gives up its author and its text here, in the query itself,
-- so no caller can reach around the rule and resurrect what somebody removed.
-- The join is what drops the author: it only matches while the parent is live.
-- name: ListMessagePreviews :many
SELECT
    m.id,
    m.channel_id,
    (m.deleted_at IS NOT NULL)::bool AS deleted,
    u.id                             AS author_id,
    u.username                       AS author_username,
    u.avatar_key                     AS author_avatar_key,
    (CASE WHEN m.deleted_at IS NULL
          THEN left(m.content, @preview_len::int)
          ELSE '' END)::text AS content,
    (CASE WHEN m.deleted_at IS NULL
          THEN char_length(m.content) > @preview_len::int
          ELSE false END)::bool AS truncated,
    (CASE WHEN m.deleted_at IS NULL
          THEN EXISTS (SELECT 1 FROM attachments a WHERE a.message_id = m.id)
          ELSE false END)::bool AS has_attachments
FROM messages m
LEFT JOIN users u ON u.id = m.author_id AND m.deleted_at IS NULL
WHERE m.id = ANY (@message_ids::uuid[]);
