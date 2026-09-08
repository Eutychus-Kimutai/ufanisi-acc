-- name: CreatePayment :one
INSERT INTO payments (idempotency_key, external_id, amount, payment_type, phone_number, client_ref, payment_ref, destination, raw_event)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payments WHERE idempotency_key = $1;

-- name: ListPayments :many
SELECT * FROM payments 
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdatePaymentStatus :exec
UPDATE payments
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: ResolvePayment :exec
UPDATE payments
SET status = $2, resolved_type = $3, resolved_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: GetUnresolvedPayments :many
SELECT * FROM payments
WHERE status = 'unresolved'
ORDER BY created_at DESC
LIMIT $1;

-- Check processing status of a payment
-- name: TryClaimPayment :one
UPDATE payments
SET STATUS = 'resolving', 
resolving_started_at = NOW(), updated_at = NOW()
WHERE idempotency_key = $1
AND (
	status = 'resolving'
	OR (
		status = 'resolving' AND
		resolving_started_at  <= NOW() - interval '20 seconds'
		)
	)
RETURNING *;

-- name: TryCompletePayment :one
UPDATE payments
SET STATUS = 'completed',
updated_at = NOW()
WHERE idempotency_key = $1
AND (
	status = 'resolving'
	)
RETURNING *;

-- name: TryFailPayment :one
UPDATE payments
SET STATUS = 'failed',
updated_at = NOW()
WHERE idempotency_key = $1
AND (
	status = 'resolving' AND
	resolving_started_at  <= NOW() - interval '20 seconds'
	)
RETURNING *;

-- name: TryUnresolvePayment :one
UPDATE payments
SET STATUS = 'unresolved',
updated_at = NOW()
WHERE idempotency_key = $1
AND (
	status = 'resolving' 
)
Returning *;

