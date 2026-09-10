-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN public_id TEXT;

UPDATE users
SET public_id = substr(translate(replace(gen_random_uuid()::text, '-', ''), '0189', 'wxyz'), 1, 16)
WHERE id <> '00000000-0000-0000-0000-000000000000';

UPDATE users
SET public_id = 'deleteduser22222'
WHERE id = '00000000-0000-0000-0000-000000000000';

ALTER TABLE users ALTER COLUMN public_id SET NOT NULL;

ALTER TABLE users
    ADD CONSTRAINT users_public_id_shape CHECK (public_id ~ '^[a-z2-7]{16}$');

CREATE UNIQUE INDEX users_public_id_key ON users (public_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX users_public_id_key;

ALTER TABLE users DROP CONSTRAINT users_public_id_shape;

ALTER TABLE users DROP COLUMN public_id;
-- +goose StatementEnd
