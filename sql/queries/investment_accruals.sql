-- name: CreateAccrualRecord :one
INSERT INTO investment_accruals (
    id,
    investment_id,
    amount,
    accrual_timestamp
) VALUES (
    gen_random_uuid(),
    $1,
    $2,
    $3
)
RETURNING *;