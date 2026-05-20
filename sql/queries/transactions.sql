-- name: CreateTransaction :one
INSERT INTO transactions (id, type, created_at, updated_at) VALUES ($1, $2, $3, $4)
RETURNING *;