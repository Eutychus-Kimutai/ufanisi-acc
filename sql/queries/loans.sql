-- name: GetLoanByLoanNumber :one
SELECT id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount, created_at, updated_at
FROM loans WHERE loan_number = $1;

-- name: GetLoanByID :one
SELECT id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount, created_at, updated_at
FROM loans WHERE id = $1;

-- name: GetLoansByClientID :many
SELECT id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount, created_at, updated_at
FROM loans WHERE client_id = $1 AND status = 'active';

-- name: UpdateLoanOutstandingAmount :exec
UPDATE loans SET outstanding_amount = $1, updated_at = NOW() WHERE id = $2;