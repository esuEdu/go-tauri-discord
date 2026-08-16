-- name: CreateInvite :one
INSERT INTO invites (code, guild_id, channel_id, inviter_id, max_uses, expires_at)
VALUES (@code, @guild_id, @channel_id, @inviter_id, @max_uses, @expires_at)
RETURNING *;

-- name: GetInvite :one
SELECT * FROM invites WHERE code = @code;

-- name: ListGuildInvites :many
SELECT * FROM invites
WHERE guild_id = @guild_id AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeInvite :exec
UPDATE invites SET revoked_at = now()
WHERE code = @code AND revoked_at IS NULL;

-- Consuming a use and validating it are the same statement on purpose. The
-- UPDATE takes a row lock, and a concurrent consumer re-evaluates the WHERE
-- against the committed row, so a max_uses = 1 invite admits exactly one
-- redeemer no matter how many arrive at once.
-- name: ConsumeInvite :one
UPDATE invites
SET uses = uses + 1
WHERE code = @code
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())
  AND (max_uses IS NULL OR uses < max_uses)
RETURNING *;

-- name: CountGuildMembers :one
SELECT count(*) FROM guild_members WHERE guild_id = @guild_id;
