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

-- name: UpdateInvestment :exec
UPDATE investments
SET principal_current = $1,
status = $2,
updated_at = $3,
client_id = $4,
annual_rate = annual_rate,
accrued_interest = $5,
next_accrual_at = $6,
last_accrual_at = $7
WHERE id = $8;

-- name: GetDueAccruals :many
SELECT * FROM investments
WHERE next_accrual_at <= $1 AND status = 'active';