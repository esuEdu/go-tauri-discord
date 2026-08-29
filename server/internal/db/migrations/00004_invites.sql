-- +goose Up
-- +goose StatementBegin
CREATE TABLE invites (
    code       TEXT PRIMARY KEY,
    guild_id   UUID        NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
    channel_id UUID        REFERENCES channels (id) ON DELETE SET NULL,
    inviter_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    max_uses   INTEGER,
    uses       INTEGER     NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT invites_max_uses_positive CHECK (max_uses IS NULL OR max_uses > 0)
);

CREATE INDEX invites_guild_idx ON invites (guild_id);
CREATE INDEX invites_inviter_idx ON invites (inviter_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE invites;
-- +goose StatementEnd
