-- name: CreateAccount :one
INSERT INTO accounts (id, name, type) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAccount :one
SELECT * FROM accounts WHERE name = $1;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountDetails :one
SELECT a.id, a.name, a.type, c.id AS client_id FROM accounts a
JOIN clients c ON a.client_id = c.id
WHERE a.name = $1;


-- name: GetCapitalAccount :one
SELECT id FROM accounts WHERE name = 'Capital Account' LIMIT 1;

