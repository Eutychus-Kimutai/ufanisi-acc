package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// Migrate creates the ledger tables that are not yet present in db.
func Migrate(ctx context.Context, db *sql.DB) error {
	const lockKey = int64(420694207)

	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto;`,

		`CREATE TABLE IF NOT EXISTS clients (
            id UUID PRIMARY KEY,
            name TEXT NOT NULL,
            client_type TEXT NOT NULL CHECK (client_type IN ('loan', 'investment')),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS transactions (
            id UUID PRIMARY KEY,
			external_id TEXT,
            type TEXT NOT NULL CHECK (type IN ('investment_deposit', 'interest_accrual', 'interest_capitalization', 'withdrawal_approved', 'withdrawal_paid', 'manual_adjustment', 'interest_income', 'interest_expense', 'loan_payment', 'loan_overpayment')),
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

		`CREATE TABLE IF NOT EXISTS payments (
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
		);`,

		`CREATE TABLE IF NOT EXISTS payment_references (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            reference TEXT UNIQUE NOT NULL,
            entity_type TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_ref TEXT;`,
		`ALTER TABLE payments ADD COLUMN IF NOT EXISTS resolving_started_at TIMESTAMPTZ;`,
		`ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_type TEXT;`,

		`CREATE TABLE IF NOT EXISTS accounts (
            id UUID PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            type TEXT NOT NULL,
			client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS loans (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
            loan_number TEXT NOT NULL UNIQUE,
			reference TEXT NOT NULL UNIQUE,
            product_type TEXT NOT NULL CHECK (product_type IN ('Personal', 'Education', 'Business')),
            status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paid_off', 'defaulted')),
            principal_amount BIGINT NOT NULL CHECK (principal_amount > 0),
            outstanding_amount BIGINT NOT NULL CHECK (outstanding_amount >= 0),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS investments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			reference TEXT NOT NULL UNIQUE,
            client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
            principal_initial BIGINT NOT NULL CHECK (principal_initial >= 0),
            principal_current BIGINT NOT NULL CHECK (principal_current >= 0),
            monthly_rate DECIMAL(5, 4) NOT NULL CHECK (monthly_rate >= 0 AND monthly_rate <= 5),
            status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'partially_withdrawn', 'closed')),
            accrued_interest BIGINT NOT NULL DEFAULT 0 CHECK (accrued_interest >= 0),
            next_accrual_at TIMESTAMPTZ NOT NULL,
            last_accrual_at TIMESTAMPTZ,
            CHECK (last_accrual_at IS NULL OR next_accrual_at > last_accrual_at),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,

		`CREATE TABLE IF NOT EXISTS entries (
            id UUID PRIMARY KEY,
            account_id UUID NOT NULL,
            transaction_id UUID NOT NULL,
			external_id TEXT NOT NULL,
            amount BIGINT NOT NULL,
            type TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	    FOREIGN KEY (account_id) REFERENCES accounts(id),
            FOREIGN KEY (transaction_id) REFERENCES transactions(id)
        );`,

		`CREATE TABLE IF NOT EXISTS investment_accruals (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            investment_id UUID NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
            accrual_timestamp TIMESTAMPTZ NOT NULL,
            amount BIGINT NOT NULL CHECK (amount >= 0),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (investment_id, accrual_timestamp)
        );`,

		`CREATE TABLE IF NOT EXISTS withdrawals_payable (
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
		`CREATE TABLE IF NOT EXISTS overpayments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
			external_id TEXT NOT NULL UNIQUE,
			amount BIGINT NOT NULL CHECK (amount > 0),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'applied', 'refunded')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE INDEX IF NOT EXISTS idx_loans_client_status ON loans(client_id, status);`,

		`CREATE INDEX IF NOT EXISTS idx_investment_accruals_investments ON investment_accruals(investment_id);`,

		`CREATE INDEX IF NOT EXISTS idx__withdrawals_payable ON withdrawals_payable(investment_id);`,

		`INSERT INTO accounts (id, name, type)
		SELECT gen_random_uuid(), 'interest_expense', 'expense'
		WHERE NOT EXISTS (
			SELECT 1 FROM accounts WHERE name = 'interest_expense'
	
		);`,

		`INSERT INTO accounts (id, name, type)
		SELECT gen_random_uuid(), 'interest_income', 'revenue'
		WHERE NOT EXISTS (
			SELECT 1 FROM accounts WHERE name = 'interest_income'
		);`,

		`INSERT INTO accounts (id, name, type)
        SELECT gen_random_uuid(), 'Capital Account', 'asset'

        WHERE NOT EXISTS (
            SELECT 1 FROM accounts WHERE name = 'Capital Account'
        );`,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Acquire advisory lock to prevent concurrent migrations
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to acquire advisory lock: %v", err)
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
