package migrations

import "database/sql"

// This file contains the database migration logic for the application.
func Migrate(db *sql.DB) error {
	// Create accounts table
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS accounts (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`)
	if err != nil {
		return err
	}
	// Create transactions table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS transactions (
		id UUID PRIMARY KEY,
		reference TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`)
	if err != nil {
		return err
	}
	// Create entries table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS entries (
		id UUID PRIMARY KEY,
		account_id UUID REFERENCES accounts(id),
		transaction_id UUID REFERENCES transactions(id),
		amount BIGINT NOT NULL,
		type TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`)
	return err
}
