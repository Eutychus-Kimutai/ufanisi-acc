-- name: CreateLoan :one
INSERT INTO loans (client_id, loan_number, reference, product_type, principal_amount, outstanding_amount, status,
    created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 'active',NOW(), NOW())
RETURNING *;

-- name: GetLoanByReference :one
SELECT * FROM loans WHERE reference = $1;

-- name: GetLoanByLoanNumber :one
SELECT *
FROM loans WHERE loan_number = $1;

-- name: GetLoanByID :one
SELECT *
FROM loans WHERE id = $1;

-- name: GetLoansByClientID :many
SELECT *
FROM loans WHERE client_id = $1 AND status = 'active';

-- name: UpdateLoanOutstandingAmount :exec
UPDATE loans SET outstanding_amount = $1, updated_at = NOW() WHERE id = $2;

-- name: UpdateLoanStatus :exec
UPDATE loans SET status = $1, updated_at = NOW() WHERE id = $2;

