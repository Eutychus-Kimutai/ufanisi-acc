-- name: CreateEntry :one
INSERT INTO entries (
    id, account_id, transaction_id, amount, type
    ) VALUES (
        $1, $2, $3, $4, $5
        )
RETURNING *;

-- name: GetEntries :many
SELECT * FROM entries WHERE account_id = $1;
