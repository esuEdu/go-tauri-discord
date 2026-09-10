-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'online';

ALTER TABLE users
    ADD CONSTRAINT users_status_known
    CHECK (status IN ('online', 'away', 'busy', 'invisible'));

ALTER TABLE users ADD COLUMN custom_status TEXT;
ALTER TABLE users ADD COLUMN bio TEXT;

ALTER TABLE users
    ADD CONSTRAINT users_custom_status_length CHECK (char_length(custom_status) <= 128);

ALTER TABLE users
    ADD CONSTRAINT users_bio_length CHECK (char_length(bio) <= 500);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT users_bio_length;
ALTER TABLE users DROP CONSTRAINT users_custom_status_length;
ALTER TABLE users DROP CONSTRAINT users_status_known;

ALTER TABLE users DROP COLUMN bio;
ALTER TABLE users DROP COLUMN custom_status;
ALTER TABLE users DROP COLUMN status;
-- +goose StatementEnd
