-- +goose Up
CREATE TABLE IF NOT EXISTS overpayments (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
external_id TEXT NOT NULL UNIQUE,
amount BIGINT NOT NULL CHECK (amount > 0),
status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'applied', 'refunded')),
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_overpayments_loan_id ON overpayments(loan_id);
CREATE INDEX IF NOT EXISTS idx_overpayments_external_id ON overpayments(external_id);

-- +goose Down
DROP TABLE IF EXISTS overpayments;

