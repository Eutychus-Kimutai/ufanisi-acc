-- name: CreateOutboxMessage :one
INSERT INTO outbox_messages (
    aggregate_type,
    aggregate_id,
    command_type,
    payload,
    status,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, 'pending', NOW(), NOW())
RETURNING *;

-- name: ClaimPendingMessages :many
update outbox_messages
set status = 'processing',
locked_at = NOW(),
locked_by = $1,
updated_at = NOW()
where id in (
    select id from outbox_messages
    where status IN ('pending', 'failed') AND
    attempts < 5
ORDER BY created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkMessageAsPublished :exec
UPDATE outbox_messages
SET status = 'published',
published_at = NOW(),
updated_at = NOW(),
last_error = NULL,
locked_at = NULL,
locked_by = NULL
WHERE id = $1;

-- name: MarkMessageAsFailed :exec
UPDATE outbox_messages
SET status = 'failed',
attempts = attempts + 1,
updated_at = NOW(),
last_error = $2,
locked_at = NULL,
locked_by = NULL
WHERE id = $1;

-- name: ReleaseStaleLocks :exec
UPDATE outbox_messages
SET status = 'failed',
last_error = 'Dispatcher lock expired',
locked_at = NULL,
locked_by = NULL,
updated_at = NOW()
WHERE status = 'processing'
AND locked_at < NOW() - INTERVAL '5 minutes';

-- name: PurgeOldMessages :execrows
WITH deleted AS (
    SELECT id FROM outbox_messages
    WHERE status = 'failed' 
    AND attempts >= 5
    AND updated_at < NOW() - (sqlc.arg(days)::int * INTERVAL '1 day')
    ORDER BY updated_at ASC
    LIMIT $1)
    DELETE FROM outbox_messages
    USING deleted
    WHERE outbox_messages.id = deleted.id;