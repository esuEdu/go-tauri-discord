-- +goose Up
-- +goose StatementBegin
CREATE TABLE guilds (
    id         UUID PRIMARY KEY,
    name       TEXT        NOT NULL,
    owner_id   UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    icon_key   TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX guilds_owner_idx ON guilds (owner_id);

CREATE TABLE channels (
    id        UUID PRIMARY KEY,
    guild_id  UUID    NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
    parent_id UUID    REFERENCES channels (id) ON DELETE SET NULL,
    kind      TEXT    NOT NULL CHECK (kind IN ('text', 'voice', 'category')),
    name      TEXT    NOT NULL,
    topic     TEXT,
    position  INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX channels_guild_position_idx ON channels (guild_id, position, id);

CREATE TABLE guild_members (
    guild_id  UUID        NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    nickname  TEXT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX guild_members_user_idx ON guild_members (user_id);

CREATE TABLE roles (
    id          UUID PRIMARY KEY,
    guild_id    UUID    NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    permissions BIGINT  NOT NULL DEFAULT 0,
    position    INTEGER NOT NULL DEFAULT 0,
    is_default  BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX roles_guild_idx ON roles (guild_id, position);
CREATE UNIQUE INDEX roles_guild_default_key ON roles (guild_id) WHERE is_default;

CREATE TABLE member_roles (
    guild_id UUID NOT NULL,
    user_id  UUID NOT NULL,
    role_id  UUID NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    PRIMARY KEY (guild_id, user_id, role_id),
    FOREIGN KEY (guild_id, user_id) REFERENCES guild_members (guild_id, user_id) ON DELETE CASCADE
);

CREATE INDEX member_roles_role_idx ON member_roles (role_id);

CREATE TABLE channel_overwrites (
    channel_id  UUID   NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    target_id   UUID   NOT NULL,
    target_type TEXT   NOT NULL CHECK (target_type IN ('role', 'member')),
    allow       BIGINT NOT NULL DEFAULT 0,
    deny        BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (channel_id, target_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE channel_overwrites;
DROP TABLE member_roles;
DROP TABLE roles;
DROP TABLE guild_members;
DROP TABLE channels;
DROP TABLE guilds;
-- +goose StatementEnd
