-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash)
VALUES (@id, @username, @email, @password_hash)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower(@email);

-- name: ListUsersByIDs :many
SELECT * FROM users WHERE id = ANY (@ids::uuid[]);

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES (@id, @user_id, @token_hash, @expires_at)
RETURNING *;

-- name: GetActiveRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token_hash = @token_hash
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = @user_id AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < now() - INTERVAL '30 days';

-- name: ReassignMessagesToUser :exec
UPDATE messages SET author_id = @new_author_id WHERE author_id = @author_id;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = @id;

-- name: SetUserAvatar :one
UPDATE users SET avatar_key = @avatar_key, updated_at = now()
WHERE id = @id
RETURNING *;
