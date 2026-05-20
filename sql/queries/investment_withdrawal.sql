-- name: CreateInvestmentWithdrawal :one
INSERT INTO investment_withdrawals (
    investment_id,
    amount,
    notice_period_months,
    requested_at,
    eligible_at,
    status
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *; 

-- name: ListEligibleWithdrawals :many
SELECT * FROM investment_withdrawals
WHERE eligible_at <= NOW()
AND status = 'pending'
ORDER BY eligible_at ASC;