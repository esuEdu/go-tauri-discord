-- +goose Up
-- +goose StatementBegin
ALTER TABLE guild_members ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

UPDATE guild_members m
SET position = ordered.rank
FROM (
    SELECT
        m2.guild_id,
        m2.user_id,
        row_number() OVER (
            PARTITION BY m2.user_id ORDER BY g.created_at, g.id
        ) - 1 AS rank
    FROM guild_members m2
    JOIN guilds g ON g.id = m2.guild_id
) ordered
WHERE m.guild_id = ordered.guild_id
  AND m.user_id = ordered.user_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE guild_members DROP COLUMN position;
-- +goose StatementEnd
