-- +goose Up
INSERT INTO accounts (id, name, type)
SELECT gen_random_uuid(), 'Investor Capital Account', 'liability'
WHERE NOT EXISTS (SELECT 1 FROM accounts WHERE name = 'Investor Capital Account');

-- +goose Down
DELETE FROM accounts WHERE name = 'Investor Capital Account';