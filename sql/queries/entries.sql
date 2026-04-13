-- name: CreateEntry :one
INSERT INTO entries (id, account_id, transaction_id, amount, type, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetEntries :many
SELECT * FROM entries WHERE account_id = $1;
