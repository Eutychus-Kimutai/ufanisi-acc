-- name: GetClientByID :one
SELECT id, name, client_type FROM clients WHERE id = $1;

