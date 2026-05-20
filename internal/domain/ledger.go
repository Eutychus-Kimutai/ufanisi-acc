package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrAccountNotFound       = sql.ErrNoRows
	ErrUnbalancedTransaction = errors.New("transaction is unbalanced")
	ErrClientNotFound        = sql.ErrNoRows
)

type LedgerService struct {
	db   *sql.DB
	repo *repository.LedgerRepository
}

func NewLedgerService(db *sql.DB, repo *repository.LedgerRepository) *LedgerService {
	return &LedgerService{db: db, repo: repo}
}

func (s *LedgerService) CreateAccount(ctx context.Context, account database.Account) error {
	err := s.repo.CreateAccount(ctx, account)
	if err != nil {
		return err
	}
	return nil
}

func (s *LedgerService) PostTransaction(ctx context.Context, transaction Transaction) error {
	var totalDebit, totalCredit int64
	// Validate transaction is balanced
	for _, entry := range transaction.Entries {
		switch entry.Type {
		case Debit:
			totalDebit += entry.Amount
		case Credit:
			totalCredit += entry.Amount
		}
	}
	if totalDebit != totalCredit {
		return ErrUnbalancedTransaction
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Verify account exists
	transactionId := uuid.New()
	createdAt := time.Now()
	// Create transaction
	err = s.repo.CreateTransactionWithTx(ctx, tx, database.Transaction{
		ID:        transactionId,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Type:      transaction.Type,
	})
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error at transaction creation: %v", err)
	}
	// Verify accounts exist
	for _, entry := range transaction.Entries {
		_, err := s.repo.GetAccount(ctx, entry.AccountId)
		if err != nil {
			tx.Rollback()
			return ErrAccountNotFound
		}
	}
	// Create entries
	for _, entry := range transaction.Entries {
		err = s.repo.CreateEntryWithTx(ctx, tx, database.Entry{
			ID:            uuid.New(),
			AccountID:     entry.AccountId,
			TransactionID: transactionId,
			Amount:        int64(entry.Amount),
			Type:          string(entry.Type),
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		})
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// CreateEntry creates a single ledger entry (not associated with a transaction)
func (s *LedgerService) CreateEntry(ctx context.Context, entry []database.CreateEntryParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Verify account exists
	for _, entry := range entry {
		_, err := s.repo.GetAccount(ctx, entry.AccountID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				tx.Rollback()
				return ErrAccountNotFound
			}
			tx.Rollback()
			return err
		}
		err = s.repo.CreateEntryWithTx(ctx, tx, database.Entry{
			ID:        uuid.New(),
			AccountID: entry.AccountID,
			Amount:    entry.Amount,
			Type:      string(entry.Type),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// GetBalance calculates the current balance for a given account
func (s *LedgerService) GetBalance(ctx context.Context, accountId uuid.UUID) (float64, error) {
	entries, err := s.repo.GetTransactionEntries(ctx, accountId)
	if err != nil {
		return 0, err
	}
	var balance float64
	for _, entry := range entries {
		switch entry.Type {
		case "Debit":
			balance += float64(entry.Amount)
		case "Credit":
			balance -= float64(entry.Amount)
		}
	}
	return balance, nil
}

// GetAccountHistory returns the transaction history for a given account
func (s *LedgerService) GetAccountHistory(ctx context.Context, accountId string) ([]Entry, error) {
	id, err := uuid.Parse(accountId)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.GetTransactionEntries(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, len(entries))
	for i, e := range entries {
		result[i] = Entry{
			TransactionId: e.TransactionID,
			AccountId:     e.AccountID,
			Amount:        e.Amount,
			Type:          EntryType(e.Type),
		}
	}
	return result, nil
}

func (s *LedgerService) GetAccount(ctx context.Context, accountId string) (database.Account, error) {
	id, err := uuid.Parse(accountId)
	if err != nil {
		return database.Account{}, err
	}
	acc, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Account{}, ErrAccountNotFound
		}
		return database.Account{}, err
	}
	return acc, nil
}

func (s *LedgerService) GetClient(ctx context.Context, clientId uuid.UUID) (database.Client, error) {
	client, err := s.repo.GetClientByID(ctx, clientId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Client{}, ErrClientNotFound
		}
		return database.Client{}, err
	}
	return client, nil
}

// Transfer funds between accounts
func (s *LedgerService) Transfer(ctx context.Context, debitAccountID, creditAccountID uuid.UUID, amount int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Verify accounts exist
	_, err = s.repo.GetAccount(ctx, debitAccountID)
	if err != nil {
		tx.Rollback()
		return ErrAccountNotFound
	}
	_, err = s.repo.GetAccount(ctx, creditAccountID)
	if err != nil {
		tx.Rollback()
		return ErrAccountNotFound
	}
	transactionID := uuid.New()
	createdAt := time.Now()
	// Create transaction
	err = s.repo.CreateTransactionWithTx(ctx, tx, database.Transaction{
		ID:        transactionID,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Type:      "Transfer",
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	// Create debit entry
	err = s.repo.CreateEntryWithTx(ctx, tx, database.Entry{
		ID:            uuid.New(),
		AccountID:     debitAccountID,
		TransactionID: transactionID,
		Amount:        amount,
		Type:          "Credit",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	// Create credit entry
	err = s.repo.CreateEntryWithTx(ctx, tx, database.Entry{
		ID:            uuid.New(),
		AccountID:     creditAccountID,
		TransactionID: transactionID,
		Amount:        amount,
		Type:          "Debit",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
