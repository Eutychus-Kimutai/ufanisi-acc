-- name: CreateEntry :one
INSERT INTO entries (
    id, account_id, transaction_id, external_id, amount, type
    ) VALUES (
        $1, $2, $3, $4, $5, $6
        )
RETURNING *;

-- name: GetEntries :many
SELECT * FROM entries WHERE account_id = $1;

-- name: GetAccountBalance :one
SELECT SUM(amount) AS balance FROM entries WHERE type = $1;
