package domain

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/Eutychus-Kimutai/ufanisi-acc/sql/migrations"
	"github.com/google/uuid"
)

var connStr = "postgres://eutychuskoech@localhost/ledger_test?sslmode=disable"

func TestPostTransaction(t *testing.T) {
	// Setup PostgreSQL in-memory database and repository
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	if err := migrations.Migrate(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}
	err = repo.CreateAccount(context.Background(), database.Account{
		ID:   revenueAccountId,
		Name: "Revenue",
		Type: "Income",
	})
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Test balanced transaction
	err = ledgerService.PostTransaction(context.Background(), Transaction{
		Reference: "Test Transaction",
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
		Reference: "Unbalanced Transaction",
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
	fmt.Printf("Results: %v\n", result)
	for i, e := range entries {
		result[i] = Entry{
			AccountId: e.AccountID,
			Amount:    e.Amount,
			Type:      EntryType(e.Type),
		}
	}
	balance, err := ledgerService.GetBalance(context.Background(), cashAccountId)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	expectedBalance := int64(100)
	if balance != expectedBalance {
		t.Errorf("Expected balance %d, got %d", expectedBalance, balance)
	}

	// Test get account history
	history, err := ledgerService.GetAccountHistory(context.Background(), cashAccountId.String())
	if err != nil {
		t.Fatalf("Failed to get account history: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 entry in account history, got %d", len(history))
	}
	if history[0].Amount != 100 || history[0].Type != Debit {
		t.Errorf("Unexpected entry in account history: %+v", history[0])
	}
}
