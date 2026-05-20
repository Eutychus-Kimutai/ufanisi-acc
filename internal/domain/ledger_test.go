package domain

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/Eutychus-Kimutai/ufanisi-acc/sql/migrations"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func testConnString(t *testing.T) string {
	t.Helper()
	// Load environment variables from .env file
	godotenv.Load("../../.env")
	DB_URL := os.Getenv("DB_URL")
	if DB_URL == "" {
		t.Fatal("DB_URL environment variable is not set")
	}
	return DB_URL
}

func TestPostTransaction(t *testing.T) {
	// Setup PostgreSQL in-memory database and repository
	connStr := testConnString(t)
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()
	require.NoError(t, migrations.Migrate(context.Background(), db), "Failed to run migrations")
	repo := repository.NewRepository(db)
	ledgerService := NewLedgerService(db, repo)

	// Create accounts
	cashAccountId := uuid.New()
	revenueAccountId := uuid.New()
	err = repo.CreateAccount(context.Background(), database.Account{
		ID:   cashAccountId,
		Name: "Cash",
		Type: "Asset",
	})
	require.NoError(t, err, "Failed to create account: Cash")
	err = repo.CreateAccount(context.Background(), database.Account{
		ID:   revenueAccountId,
		Name: "Revenue",
		Type: "Income",
	})
	require.NoError(t, err, "Failed to create account: Revenue")

	defer db.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", cashAccountId)
	defer db.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", revenueAccountId)

	// Test balanced transaction
	err = ledgerService.PostTransaction(context.Background(), Transaction{
		Type: "Test Transaction",
		Entries: []Entry{
			{AccountId: cashAccountId, Type: Debit, Amount: 100},
			{AccountId: revenueAccountId, Type: Credit, Amount: 100},
		},
	})
	if err != nil {
		t.Errorf("Expected balanced transaction to succeed, got error: %v", err)
	}

	// Test unbalanced transaction
	err = ledgerService.PostTransaction(context.Background(), Transaction{
		Type: "Unbalanced Transaction",
		Entries: []Entry{
			{AccountId: uuid.New(), Type: Debit, Amount: 100},
			{AccountId: uuid.New(), Type: Credit, Amount: 50},
		},
	})
	if err != ErrUnbalancedTransaction {
		t.Errorf("Expected unbalanced transaction to fail with ErrUnbalancedTransaction, got: %v", err)
	}

	// Test get balance
	entries, err := repo.GetTransactionEntries(context.Background(), cashAccountId)
	if err != nil {
		t.Fatalf("Failed to get transaction entries: %v", err)
	}
	result := make([]Entry, len(entries))
	for i, e := range entries {
		result[i] = Entry{
			AccountId: e.AccountID,
			Amount:    e.Amount,
			Type:      EntryType(e.Type),
		}
	}
	balance, err := ledgerService.GetBalance(context.Background(), cashAccountId)
	require.NoError(t, err, "Failed to get account balance")
	expectedBalance := 100.0
	require.Equal(t, expectedBalance, balance, "Expected balance to be %.2f, got %.2f", expectedBalance, balance)

	// Test get account history
	history, err := ledgerService.GetAccountHistory(context.Background(), cashAccountId.String())
	require.NoError(t, err, "Failed to get account history")
	require.Len(t, history, 1, "Expected account history to have 1 entry")
	require.Equal(t, cashAccountId, history[0].AccountId, "Expected account ID to match")
	require.Equal(t, int64(100.0), history[0].Amount, "Expected amount to match")
	require.Equal(t, Debit, history[0].Type, "Expected entry type to be Debit")

	// Test transfer between accounts
	err = ledgerService.Transfer(context.Background(), cashAccountId, revenueAccountId, 50)
	if err != nil {
		t.Fatalf("Failed to transfer between accounts: %v", err)
	}
	cashBalance, err := ledgerService.GetBalance(context.Background(), cashAccountId)
	require.NoError(t, err, "Failed to get cash account balance")
	require.Equal(t, 50.0, cashBalance, "Expected cash account balance to be 50 after transfer")
}
