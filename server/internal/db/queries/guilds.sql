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



-- name: UpsertChannelOverwrite :exec
INSERT INTO channel_overwrites (channel_id, target_id, target_type, allow, deny)
VALUES (@channel_id, @target_id, @target_type, @allow, @deny)
ON CONFLICT (channel_id, target_id)
DO UPDATE SET allow = EXCLUDED.allow, deny = EXCLUDED.deny;

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
        SELECT json_agg(json_build_object('id', r.id, 'permissions', r.permissions)
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
        SELECT json_agg(json_build_object('id', r.id, 'permissions', r.permissions)
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
