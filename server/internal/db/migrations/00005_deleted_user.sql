-- +goose Up
-- +goose StatementBegin
INSERT INTO users (id, username, email, password_hash)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'Deleted User',
    'deleted-user@invalid.localhost',
    ''
)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000000';
-- +goose StatementEnd
