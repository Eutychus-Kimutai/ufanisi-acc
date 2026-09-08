-- name: CreateTransaction :one
INSERT INTO transactions (id, type, external_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)
RETURNING *;
