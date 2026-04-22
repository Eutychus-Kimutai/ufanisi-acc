-- +goose Up
CREATE TABLE IF NOT EXISTS unresolved_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_ref TEXT NOT NULL,
    amount BIGINT NOT NULL,
    payment_channel TEXT NOT NULL,
    external_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    raw_event JSONB NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS unresolved_payments;