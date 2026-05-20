-- +goose Up
CREATE TABLE IF NOT EXISTS withdrawals_payable (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    investment_id UUID NOT NULL 
        REFERENCES investments(id) ON DELETE CASCADE,
    
    amount BIGINT NOT NULL CHECK (amount >= 0),

    notice_period_months INT NOT NULL CHECK (notice_period_months IN (1,2)),

    requested_at TIMESTAMPTZ NOT NULL,
    eligible_at TIMESTAMPTZ NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'eligible', 'processed', 'cancelled')),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawal_payable ON withdrawals_payable(investment_id);

-- +goose Down
DROP TABLE IF EXISTS withdrawals_payable;
