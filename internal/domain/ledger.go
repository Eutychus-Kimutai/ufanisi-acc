package domain

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrAccountNotFound       = sql.ErrNoRows
	ErrUnbalancedTransaction = errors.New("transaction is unbalanced")
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
	log.Printf("Created account: %+v\n", account)
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
	log.Printf("Started transaction: %s\n", transaction.Id)
	// Verify account exists
	transactionId := uuid.New()
	createdAt := time.Now()
	// Create transaction
	err = s.repo.CreateTransaction(ctx, database.Transaction{
		ID:        transactionId,
		Reference: transaction.Reference,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	log.Printf("Created transaction: %+v\n", transaction)
	// Verify accounts exist
	for _, entry := range transaction.Entries {
		_, err := s.repo.GetAccount(ctx, entry.AccountId)
		if err != nil {
			tx.Rollback()
			return ErrAccountNotFound
		}
		log.Printf("Verified account exists: %s\n", entry.AccountId)
	}
	// Create entries
	for _, entry := range transaction.Entries {
		err = s.repo.CreateEntry(ctx, database.Entry{
			ID:            uuid.New(),
			AccountID:     entry.AccountId,
			TransactionID: transactionId,
			Amount:        entry.Amount,
			Type:          string(entry.Type),
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		})
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	log.Printf("Created entries for transaction: %s\n", transactionId)
	err = tx.Commit()
	if err != nil {
		return err
	}
	log.Printf("Committed transaction: %s\n", transactionId)
	return nil
}

// Create entry
func (s *LedgerService) CreateEntry(ctx context.Context, entry database.Entry) error {
	// Verify account exists
	_, err := s.repo.GetAccount(ctx, entry.AccountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		}
		return err
	}
	err = s.repo.CreateEntry(ctx, entry)
	if err != nil {
		return err
	}
	log.Printf("Created entry: %+v\n", entry)
	return nil
}

// Get balance
func (s *LedgerService) GetBalance(ctx context.Context, accountId uuid.UUID) (int64, error) {
	entries, err := s.repo.GetTransactionEntries(ctx, accountId)
	if err != nil {
		return 0, err
	}
	var balance int64
	for _, entry := range entries {
		switch entry.Type {
		case "Debit":
			balance += entry.Amount
		case "Credit":
			balance -= entry.Amount
		}
	}
	return balance, nil
}

// Get account history
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
	log.Printf("Retrieved account: %+v\n", acc)
	return acc, nil
}
