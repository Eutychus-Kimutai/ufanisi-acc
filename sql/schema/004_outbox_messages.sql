-- +goose Up
CREATE TABLE outbox_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,

    command_type TEXT NOT NULL,
    payload JSONB NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'processing', 'published', 'failed')),

    attempts INT NOT NULL DEFAULT 0
    CHECK (attempts >= 0 AND attempts <= 5),


    locked_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    locked_by TEXT,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_dispatch ON outbox_messages (status, created_at);

CREATE INDEX idx_outbox_aggregate ON outbox_messages (aggregate_type, aggregate_id);

-- +goose Down
DROP TABLE outbox_messages;

