-- name: CreateUnresolvedPayment :exec
INSERT INTO unresolved_payments (client_ref, amount, payment_channel, external_id, reason, raw_event)
VALUES ($1, $2, $3, $4, $5, $6);