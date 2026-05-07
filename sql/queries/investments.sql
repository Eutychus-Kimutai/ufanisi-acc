-- name: CreateInvestment :one
INSERT INTO investments (
    id,
    client_id,
    principal_initial,
    principal_current,
    annual_rate,
    status,
    accrued_interest,
    next_accrual_at,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $2,
    0.3,
    'active',
    0,
    $3,
    NOW(),
    NOW()
)
RETURNING *;

-- name: GetInvestmentByID :one
SELECT * FROM investments WHERE id = $1;

-- name: UpdateInvestmentAccrual :exec
UPDATE investments
SET accrued_interest = accrued_interest + $1, 
next_accrual_at = $2, 
last_accrual_at = $3,
updated_at = $4
WHERE id = $5;