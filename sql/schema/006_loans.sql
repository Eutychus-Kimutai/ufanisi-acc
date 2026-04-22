-- +goose Up
CREATE TABLE IF NOT EXISTS loans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    loan_number TEXT NOT NULL UNIQUE,
    product_type TEXT NOT NULL CHECK (product_type IN ('Personal', 'Education', 'Business')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paid_off', 'defaulted')),
    principal_amount BIGINT NOT NULL CHECK (principal_amount > 0),
    outstanding_amount BIGINT NOT NULL CHECK (outstanding_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_loans_client_status ON loans(client_id, status);

-- +goose Down
DROP TABLE IF EXISTS loans;