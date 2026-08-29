-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN discriminator TEXT NOT NULL DEFAULT '0000';

UPDATE users
SET discriminator = lpad(((random() * 9998)::int + 1)::text, 4, '0')
WHERE id <> '00000000-0000-0000-0000-000000000000';

ALTER TABLE users ALTER COLUMN discriminator DROP DEFAULT;

ALTER TABLE users
    ADD CONSTRAINT users_discriminator_shape CHECK (discriminator ~ '^[0-9]{4}$');

DROP INDEX users_username_lower_key;

CREATE UNIQUE INDEX users_username_lower_discriminator_key
    ON users (lower(username), discriminator);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX users_username_lower_discriminator_key;

ALTER TABLE users DROP CONSTRAINT users_discriminator_shape;

ALTER TABLE users DROP COLUMN discriminator;

CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));
-- +goose StatementEnd
