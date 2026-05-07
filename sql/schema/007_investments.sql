-- +goose Up
CREATE TABLE IF NOT EXISTS investments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,

    principal_initial BIGINT NOT NULL CHECK (principal_initial >= 0),
    principal_current BIGINT NOT NULL CHECK (principal_current >= 0),
    CHECK (principal_current <= principal_initial),

    annual_rate DECIMAL(5, 4) NOT NULL 
        CHECK (annual_rate >= 0 AND annual_rate <= 1),

    status TEXT NOT NULL DEFAULT 'active' 
        CHECK (status IN ('active', 'partially_withdrawn', 'closed')),

    accrued_interest BIGINT NOT NULL DEFAULT 0 
        CHECK (accrued_interest >= 0),

    next_accrual_at TIMESTAMPTZ NOT NULL,
    last_accrual_at TIMESTAMPTZ,

    CHECK (
        last_accrual_at IS NULL 
        OR next_accrual_at > last_accrual_at
    ),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS investments;
