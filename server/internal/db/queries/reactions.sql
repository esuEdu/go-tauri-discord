-- name: AddReaction :execrows
INSERT INTO message_reactions (message_id, user_id, emoji)
VALUES (@message_id, @user_id, @emoji)
ON CONFLICT DO NOTHING;

-- name: RemoveReaction :execrows
DELETE FROM message_reactions
WHERE message_id = @message_id AND user_id = @user_id AND emoji = @emoji;

-- name: CountReactionKinds :one
SELECT count(DISTINCT emoji)::int FROM message_reactions WHERE message_id = @message_id;

-- name: ListReactionsForMessages :many
SELECT
    message_id,
    emoji,
    count(*)::int                    AS count,
    bool_or(user_id = @viewer_id)    AS mine
FROM message_reactions
WHERE message_id = ANY (@message_ids::uuid[])
GROUP BY message_id, emoji
ORDER BY message_id, min(created_at);

-- name: ListReactors :many
SELECT u.id, u.username, u.discriminator, u.avatar_key
FROM message_reactions r
JOIN users u ON u.id = r.user_id
WHERE r.message_id = @message_id AND r.emoji = @emoji
ORDER BY r.created_at;
