-- name: BanUser :one
INSERT INTO guild_bans (guild_id, user_id, banned_by, reason)
VALUES (@guild_id, @user_id, @banned_by, @reason)
ON CONFLICT (guild_id, user_id)
DO UPDATE SET banned_by = EXCLUDED.banned_by, reason = EXCLUDED.reason
RETURNING *;

-- name: GetGuildBan :one
SELECT * FROM guild_bans WHERE guild_id = @guild_id AND user_id = @user_id;

-- name: ListGuildBans :many
SELECT b.*, u.username
FROM guild_bans b
JOIN users u ON u.id = b.user_id
WHERE b.guild_id = @guild_id
ORDER BY b.created_at DESC, b.user_id;

-- name: DeleteGuildBan :execrows
DELETE FROM guild_bans WHERE guild_id = @guild_id AND user_id = @user_id;
