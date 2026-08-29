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
ORDER BY m.position, g.created_at, g.id;

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

-- name: SetChannelPosition :exec
UPDATE channels SET position = @position WHERE id = @id AND guild_id = @guild_id;

-- name: SetChannelParent :exec
UPDATE channels SET parent_id = @parent_id WHERE id = @id AND guild_id = @guild_id;

-- name: UpdateChannel :one
UPDATE channels SET
    name  = COALESCE(sqlc.narg('name'), name),
    topic = CASE WHEN @clear_topic::bool THEN NULL ELSE COALESCE(sqlc.narg('topic'), topic) END
WHERE id = @id
RETURNING *;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = @id;

-- name: AddGuildMember :one
INSERT INTO guild_members (guild_id, user_id, nickname, position)
VALUES (
    @guild_id,
    @user_id,
    @nickname,
    COALESCE(
        (SELECT MAX(position) + 1 FROM guild_members WHERE user_id = @user_id),
        0
    )
)
ON CONFLICT (guild_id, user_id) DO UPDATE SET nickname = EXCLUDED.nickname
RETURNING *;

-- name: SetGuildPosition :exec
UPDATE guild_members SET position = @position
WHERE user_id = @user_id AND guild_id = @guild_id;

-- name: CountGuildsForUser :one
SELECT count(*) FROM guild_members WHERE user_id = @user_id;

-- name: GetGuildMember :one
SELECT * FROM guild_members WHERE guild_id = @guild_id AND user_id = @user_id;

-- name: ListGuildMembers :many
SELECT m.*, u.username, u.discriminator, u.avatar_key
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

-- name: GetRole :one
SELECT * FROM roles WHERE id = @id;

-- name: UpdateRole :one
UPDATE roles SET
    name        = COALESCE(sqlc.narg('name'), name),
    permissions = COALESCE(sqlc.narg('permissions'), permissions),
    position    = COALESCE(sqlc.narg('position'), position)
WHERE id = @id
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = @id;

-- name: AssignRole :exec
INSERT INTO member_roles (guild_id, user_id, role_id)
VALUES (@guild_id, @user_id, @role_id)
ON CONFLICT DO NOTHING;

-- name: UnassignRole :exec
DELETE FROM member_roles
WHERE guild_id = @guild_id AND user_id = @user_id AND role_id = @role_id;

-- name: ListMemberRoles :many
SELECT r.* FROM roles r
JOIN member_roles mr ON mr.role_id = r.id
WHERE mr.guild_id = @guild_id AND mr.user_id = @user_id
ORDER BY r.position DESC, r.id;

-- name: UpsertChannelOverwrite :exec
INSERT INTO channel_overwrites (channel_id, target_id, target_type, allow, deny)
VALUES (@channel_id, @target_id, @target_type, @allow, @deny)
ON CONFLICT (channel_id, target_id)
DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny;

-- name: DeleteChannelOverwrite :exec
DELETE FROM channel_overwrites
WHERE channel_id = @channel_id AND target_id = @target_id;

-- name: ListChannelOverwrites :many
SELECT * FROM channel_overwrites WHERE channel_id = @channel_id ORDER BY target_id;

-- name: DeleteOverwritesForTarget :exec
DELETE FROM channel_overwrites o
USING channels c
WHERE c.id = o.channel_id AND c.guild_id = @guild_id AND o.target_id = @target_id;

-- name: ListGuildsOwnedBy :many
SELECT * FROM guilds WHERE owner_id = @owner_id;

-- name: NextGuildOwner :one
SELECT m.user_id
FROM guild_members m
LEFT JOIN member_roles mr ON mr.guild_id = m.guild_id AND mr.user_id = m.user_id
LEFT JOIN roles r ON r.id = mr.role_id
WHERE m.guild_id = @guild_id AND m.user_id <> @leaving_id
GROUP BY m.user_id, m.joined_at
ORDER BY COALESCE(MAX(r.position), -1) DESC, m.joined_at, m.user_id
LIMIT 1;

-- name: TransferGuildOwnership :exec
UPDATE guilds SET owner_id = @owner_id WHERE id = @id;

-- name: ResolveChannelAccess :one
SELECT
    sqlc.embed(c),
    g.owner_id,
    (m.user_id IS NOT NULL)::bool AS is_member,
    COALESCE((
        SELECT json_agg(json_build_object('id', r.id,
                                          'permissions', r.permissions,
                                          'position', r.position)
                        ORDER BY r.position DESC, r.id)
        FROM roles r
        WHERE r.guild_id = c.guild_id
          AND (
            r.is_default
            OR EXISTS (
              SELECT 1 FROM member_roles mr
              WHERE mr.role_id = r.id AND mr.user_id = @user_id
            )
          )
    ), '[]'::json)::text AS roles,
    COALESCE((
        SELECT json_agg(json_build_object('target_id', o.target_id,
                                          'target_type', o.target_type,
                                          'allow', o.allow,
                                          'deny', o.deny))
        FROM channel_overwrites o
        WHERE o.channel_id = c.id
    ), '[]'::json)::text AS overwrites
FROM channels c
JOIN guilds g ON g.id = c.guild_id
LEFT JOIN guild_members m ON m.guild_id = c.guild_id AND m.user_id = @user_id
WHERE c.id = @channel_id;

-- name: ResolveGuildAccess :one
SELECT
    g.owner_id,
    (m.user_id IS NOT NULL)::bool AS is_member,
    COALESCE((
        SELECT json_agg(json_build_object('id', r.id,
                                          'permissions', r.permissions,
                                          'position', r.position)
                        ORDER BY r.position DESC, r.id)
        FROM roles r
        WHERE r.guild_id = g.id
          AND (
            r.is_default
            OR EXISTS (
              SELECT 1 FROM member_roles mr
              WHERE mr.role_id = r.id AND mr.user_id = @user_id
            )
          )
    ), '[]'::json)::text AS roles
FROM guilds g
LEFT JOIN guild_members m ON m.guild_id = g.id AND m.user_id = @user_id
WHERE g.id = @guild_id;

-- name: ListGuildOverwrites :many
SELECT o.* FROM channel_overwrites o
JOIN channels c ON c.id = o.channel_id
WHERE c.guild_id = @guild_id;

-- name: SetGuildIcon :one
UPDATE guilds SET icon_key = @icon_key
WHERE id = @id
RETURNING *;
