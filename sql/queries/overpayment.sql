-- name: CreateOverpayment :one
INSERT INTO overpayments (
    loan_id,
    external_id,
    amount,
    status,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    'pending',
    NOW(),
    NOW()
)
RETURNING *;

-- name: GetOverpaymentByExternalID :one
SELECT * FROM overpayments WHERE external_id = $1;

-- name: UpdateOverpaymentStatus :exec
UPDATE overpayments SET status = $1, updated_at = NOW() WHERE id = $2; 

-- name: UpdateOverpaymentAmount :exec
UPDATE overpayments SET amount = $1, updated_at = NOW() WHERE id = $2;


