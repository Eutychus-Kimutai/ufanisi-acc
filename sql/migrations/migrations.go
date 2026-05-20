package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,

		`CREATE TABLE IF NOT EXISTS accounts (
            id UUID PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            type TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS transactions (
            id UUID PRIMARY KEY,
            type TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS entries (
            id UUID PRIMARY KEY,
            account_id UUID NOT NULL,
            transaction_id UUID NOT NULL,
            amount BIGINT NOT NULL,
            type TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            FOREIGN KEY (account_id) REFERENCES accounts(id),
            FOREIGN KEY (transaction_id) REFERENCES transactions(id)
        );`,

		`CREATE TABLE IF NOT EXISTS clients (
            id UUID PRIMARY KEY,
            name TEXT NOT NULL,
            client_type TEXT NOT NULL CHECK (client_type IN ('loan', 'investment')),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS unresolved_payments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            client_ref TEXT NOT NULL,
            amount BIGINT NOT NULL,
            payment_channel TEXT NOT NULL,
            external_id TEXT NOT NULL,
            reason TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            raw_event JSONB NOT NULL,
            UNIQUE (external_id)
        );`,

		`CREATE TABLE IF NOT EXISTS loans (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
            loan_number TEXT NOT NULL UNIQUE,
            product_type TEXT NOT NULL CHECK (product_type IN ('Personal', 'Education', 'Business')),
            status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paid_off', 'defaulted')),
            principal_amount BIGINT NOT NULL CHECK (principal_amount > 0),
            outstanding_amount BIGINT NOT NULL CHECK (outstanding_amount >= 0),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE INDEX IF NOT EXISTS idx_loans_client_status ON loans(client_id, status);`,

		`CREATE TABLE IF NOT EXISTS investments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
            principal_initial BIGINT NOT NULL CHECK (principal_initial >= 0),
            principal_current BIGINT NOT NULL CHECK (principal_current >= 0),
            CHECK (principal_current <= principal_initial),
            annual_rate DECIMAL(5, 4) NOT NULL CHECK (annual_rate >= 0 AND annual_rate <= 1),
            status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'partially_withdrawn', 'closed')),
            accrued_interest BIGINT NOT NULL DEFAULT 0 CHECK (accrued_interest >= 0),
            next_accrual_at TIMESTAMPTZ NOT NULL,
            last_accrual_at TIMESTAMPTZ,
            CHECK (last_accrual_at IS NULL OR next_accrual_at > last_accrual_at),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS investment_accruals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            investment_id UUID NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
            accrual_timestamp TIMESTAMPTZ NOT NULL,
            amount BIGINT NOT NULL CHECK (amount >= 0),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (investment_id, accrual_timestamp)
        );`,

		`CREATE INDEX IF NOT EXISTS idx_investment_accruals_investments ON investment_accruals(investment_id);`,

		`CREATE TABLE IF NOT EXISTS investment_withdrawals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            investment_id UUID NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
            amount BIGINT NOT NULL CHECK (amount >= 0),
            notice_period_months INT NOT NULL CHECK (notice_period_months IN (1,2)),
            requested_at TIMESTAMPTZ NOT NULL,
            eligible_at TIMESTAMPTZ NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'eligible', 'processed', 'cancelled')),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE INDEX IF NOT EXISTS idx_investment_withdrawals_investments ON investment_withdrawals(investment_id);`,

		`INSERT INTO accounts (id, name, type)
        SELECT gen_random_uuid(), 'Capital Account', 'liability'
        WHERE NOT EXISTS (
            SELECT 1 FROM accounts WHERE name = 'Capital Account'
        );`,

		`INSERT INTO accounts (id, name, type)
        SELECT gen_random_uuid(), 'Investor Funds Account', 'liability'
        WHERE NOT EXISTS (
            SELECT 1 FROM accounts WHERE name = 'Investor Funds Account'
        );`,

		`CREATE TABLE IF NOT EXISTS outbox_messages (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            aggregate_type TEXT NOT NULL,
            aggregate_id UUID NOT NULL,

            command_type TEXT NOT NULL,
            payload JSONB NOT NULL,

            status TEXT NOT NULL DEFAULT 'pending'
            CHECK (status IN ('pending', 'processing', 'published', 'failed')),

            attempts INT NOT NULL DEFAULT 0
            CHECK (attempts >= 0),


            locked_at TIMESTAMPTZ,
            published_at TIMESTAMPTZ,
            locked_by TEXT,
            last_error TEXT,

            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute statement: %v, error: %v", stmt, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}
