-- name: CreatePaymentReference :one
INSERT INTO payment_references (reference, entity_type)
VALUES ($1, $2 )
RETURNING *;

-- name: GetPaymentReference :one
SELECT *
FROM payment_references
WHERE reference = $1;


