-- name: CreateAccount :one
INSERT INTO accounts (id, name, type) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAccount :one
SELECT id, name, type, created_at, updated_at FROM accounts WHERE id = $1;
