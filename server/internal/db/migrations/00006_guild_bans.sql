-- +goose Up
-- +goose StatementBegin
CREATE TABLE guild_bans (
    guild_id   UUID        NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    banned_by  UUID        REFERENCES users (id) ON DELETE SET NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX guild_bans_user_idx ON guild_bans (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE guild_bans;
-- +goose StatementEnd
