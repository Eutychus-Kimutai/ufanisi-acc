-- +goose Up
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT UNIQUE NOT NULL,
	external_id TEXT NOT NULL,
	amount BIGINT NOT NULL,
	payment_type TEXT NOT NULL,
	phone_number TEXT NOT NULL,
	client_ref TEXT NOT NULL,
	payment_ref TEXT NOT NULL,
	destination TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'received',
	CHECK (status IN ('received', 'resolving', 'unresolved', 'completed', 'failed')),
	resolved_type TEXT,
	resolving_started_at TIMESTAMPTZ,
	raw_event JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	resolved_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_status ON payments (status, created_at);
CREATE INDEX idx_payments_external_id ON payments (external_id);
-- +goose Down
DROP TABLE payments;

