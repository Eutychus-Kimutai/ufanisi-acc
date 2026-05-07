-- +goose Up
CREATE TABLE IF NOT EXISTS investment_accruals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    investment_id UUID NOT NULL 
        REFERENCES investments(id) ON DELETE CASCADE,
    
    accrual_timestamp TIMESTAMPTZ NOT NULL,
    amount BIGINT NOT NULL CHECK (amount >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (investment_id, accrual_timestamp)
);

CREATE INDEX idx_investment_accruals_investments ON investment_accruals(investment_id);

-- +goose Down
DROP TABLE IF EXISTS investment_accruals;