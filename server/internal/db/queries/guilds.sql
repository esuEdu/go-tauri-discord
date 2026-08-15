-- name: CreateGuild :one
INSERT INTO guilds (id, name, owner_id)
VALUES (@id, @name, @owner_id)
RETURNING *;

-- name: GetGuild :one
SELECT * FROM guilds WHERE id = @id;

-- name: ListGuildsForUser :many
SELECT g.* FROM guilds g
JOIN guild_members m ON m.guild_id = g.id
WHERE m.user_id = @user_id
ORDER BY g.created_at;

-- name: DeleteGuild :exec
DELETE FROM guilds WHERE id = @id;

-- name: CreateChannel :one
INSERT INTO channels (id, guild_id, parent_id, kind, name, topic, position)
VALUES (@id, @guild_id, @parent_id, @kind, @name, @topic, @position)
RETURNING *;

-- name: GetChannel :one
SELECT * FROM channels WHERE id = @id;

-- name: ListChannels :many
SELECT * FROM channels WHERE guild_id = @guild_id ORDER BY position, id;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = @id;

-- name: AddGuildMember :one
INSERT INTO guild_members (guild_id, user_id, nickname)
VALUES (@guild_id, @user_id, @nickname)
ON CONFLICT (guild_id, user_id) DO UPDATE SET nickname = EXCLUDED.nickname
RETURNING *;

-- name: GetGuildMember :one
SELECT * FROM guild_members WHERE guild_id = @guild_id AND user_id = @user_id;

-- name: ListGuildMembers :many
SELECT m.*, u.username, u.avatar_key
FROM guild_members m
JOIN users u ON u.id = m.user_id
WHERE m.guild_id = @guild_id
ORDER BY u.username;

-- name: RemoveGuildMember :exec
DELETE FROM guild_members WHERE guild_id = @guild_id AND user_id = @user_id;

-- name: ListGuildMemberIDs :many
SELECT user_id FROM guild_members WHERE guild_id = @guild_id;

-- name: CreateRole :one
INSERT INTO roles (id, guild_id, name, permissions, position, is_default)
VALUES (@id, @guild_id, @name, @permissions, @position, @is_default)
RETURNING *;

-- name: ListRoles :many
SELECT * FROM roles WHERE guild_id = @guild_id ORDER BY position DESC, id;

-- name: AssignRole :exec
INSERT INTO member_roles (guild_id, user_id, role_id)
VALUES (@guild_id, @user_id, @role_id)
ON CONFLICT DO NOTHING;

-- name: ListEffectiveRoles :many
SELECT r.* FROM roles r
WHERE r.guild_id = @guild_id
  AND (
    r.is_default
    OR EXISTS (
      SELECT 1 FROM member_roles mr
      WHERE mr.role_id = r.id AND mr.user_id = @user_id
    )
  )
ORDER BY r.position DESC, r.id;

-- name: ListChannelOverwrites :many
SELECT * FROM channel_overwrites WHERE channel_id = @channel_id;

-- name: UpsertChannelOverwrite :exec
INSERT INTO channel_overwrites (channel_id, target_id, target_type, allow, deny)
VALUES (@channel_id, @target_id, @target_type, @allow, @deny)
ON CONFLICT (channel_id, target_id)
DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny;
