-- name: CreateInvestmentWithdrawal :one
INSERT INTO withdrawals_payable (
    investment_id,
    amount,
    notice_period_months,
    requested_at,
    eligible_at,
    status
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *; 

-- name: ListEligibleWithdrawals :many
SELECT * FROM withdrawals_payable
WHERE eligible_at <= NOW()
AND status = 'pending'
ORDER BY eligible_at ASC;

-- name: UpdateWithdrawalStatus :exec
UPDATE withdrawals_payable
SET status = $1,
updated_at = NOW()
WHERE id = $2
AND status = 'pending';
