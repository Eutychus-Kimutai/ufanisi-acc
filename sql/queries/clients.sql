-- name: GetClientByID :one
SELECT * FROM clients WHERE id = $1;

