package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	db          *sql.DB
	ledgerRepo  *repository.LedgerRepository
	clientRepo  *repository.ClientRepository
	accountRepo *repository.AccountsRepository
}

func NewLedgerService(db *sql.DB, ledgerRepo *repository.LedgerRepository, clientRepo *repository.ClientRepository) *LedgerService {
	return &LedgerService{db: db, ledgerRepo: ledgerRepo, clientRepo: clientRepo, accountRepo: repository.NewAccountsRepository(db)}
}
func (s *LedgerService) WithTx(tx *sql.Tx) *LedgerService {
	return &LedgerService{
		db:         s.db,
		ledgerRepo: s.ledgerRepo.WithTx(tx),
	}
}

func (s *LedgerService) CreateAccount(ctx context.Context, account database.Account) error {
	err := s.ledgerRepo.CreateAccount(ctx, account)
	if err != nil {
		return err
	}
	return nil
}

func (s *LedgerService) PostTransaction(ctx context.Context, transaction Transaction) error {
	var totalDebit, totalCredit int64
	// Validate transaction is balanced
	if len(transaction.Entries) == 0 {
		return fmt.Errorf("transaction must have at least one entry")
	}
	for _, entry := range transaction.Entries {
		switch entry.Type {
		case Debit:
			totalDebit += entry.Amount
		case Credit:
			totalCredit += entry.Amount
		default:
			return fmt.Errorf("transaction entries not balanced: %s", entry.Type)
		}
		if entry.Amount <= 0 {
			return fmt.Errorf("entry amount must be greater than zero")
		}
	}

	if totalDebit != totalCredit {
		return ErrUnbalancedTransaction
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	transactionId := uuid.New()
	if transaction.Id != uuid.Nil {
		transactionId = transaction.Id
	}

	createdAt := time.Now()
	// Create transaction
	err = s.ledgerRepo.CreateTransactionWithTx(ctx, tx, database.Transaction{
		ID:         transactionId,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		ExternalID: sql.NullString{String: transaction.ExternalId, Valid: true},
		Type:       transaction.Type,
	})
	if err != nil {
		return fmt.Errorf("error at transaction creation: %v", err)
	}
	// Verify accounts exist
	for _, entry := range transaction.Entries {
		a, err := s.ledgerRepo.GetAccountByID(ctx, entry.AccountId)
		if err != nil {
			return fmt.Errorf("error at account verification: %v", err)
		}
		log.Printf("account verified: %s, type: %s", a.Name, a.Type)
	}
	// Create entries
	for _, entry := range transaction.Entries {
		err = s.ledgerRepo.CreateEntryWithTx(ctx, tx, database.Entry{
			ID:            uuid.New(),
			AccountID:     entry.AccountId,
			TransactionID: transactionId,
			ExternalID:    entry.ExternalId,
			Amount:        entry.Amount,
			Type:          string(entry.Type),
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		})
		if err != nil {
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
	defer tx.Rollback()
	// Verify account exists
	for _, entry := range entry {
		_, err := s.ledgerRepo.GetAccountByID(ctx, entry.AccountID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrAccountNotFound
			}
			tx.Rollback()
			return err
		}
		err = s.ledgerRepo.CreateEntryWithTx(ctx, tx, database.Entry{
			ID:        uuid.New(),
			AccountID: entry.AccountID,
			Amount:    entry.Amount,
			Type:      string(entry.Type),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		if err != nil {
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
func (s *LedgerService) GetBalance(ctx context.Context, accountType string) (int64, error) {
	balance, err := s.ledgerRepo.GetAccountBalance(ctx, accountType)
	if err != nil {
		return 0, err
	}
	return balance, nil

}

// GetAccountHistory returns the transaction history for a given account
func (s *LedgerService) GetAccountHistory(ctx context.Context, accountId string) ([]Entry, error) {
	id, err := uuid.Parse(accountId)
	if err != nil {
		return nil, err
	}
	entries, err := s.ledgerRepo.GetTransactionEntries(ctx, id)
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

func (s *LedgerService) GetAccount(ctx context.Context, accountRef string) (database.Account, error) {
	acc, err := s.ledgerRepo.GetAccount(ctx, accountRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Account{}, ErrAccountNotFound
		}
		return database.Account{}, err
	}
	return acc, nil
}
func (s *LedgerService) GetAccountDetails(ctx context.Context, accountRef string) (database.GetAccountDetailsRow, error) {
	acc, err := s.accountRepo.GetAccountDetails(ctx, accountRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.GetAccountDetailsRow{}, ErrAccountNotFound
		}
		return database.GetAccountDetailsRow{}, err
	}
	return acc, nil
}

func (s *LedgerService) GetClient(ctx context.Context, clientId uuid.UUID) (database.Client, error) {
	client, err := s.clientRepo.GetClientByID(ctx, clientId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Client{}, ErrClientNotFound
		}
		return database.Client{}, err
	}
	return client, nil
}

// Transfer funds between accounts
func (s *LedgerService) Transfer(ctx context.Context, debitAccountID, creditAccountID uuid.UUID, amount int64, investmentType string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		return err
	}
	// Verify accounts exist
	_, err = s.ledgerRepo.GetAccountByID(ctx, debitAccountID)
	if err != nil {
		return ErrAccountNotFound
	}
	_, err = s.ledgerRepo.GetAccountByID(ctx, creditAccountID)
	if err != nil {
		return ErrAccountNotFound
	}
	transactionID := uuid.New()
	createdAt := time.Now()
	// Create transaction
	err = s.ledgerRepo.CreateTransactionWithTx(ctx, tx, database.Transaction{
		ID:        transactionID,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Type:      investmentType,
	})
	if err != nil {
		return err
	}
	// Create debit entry
	err = s.ledgerRepo.CreateEntryWithTx(ctx, tx, database.Entry{
		ID:            uuid.New(),
		AccountID:     debitAccountID,
		TransactionID: transactionID,
		Amount:        amount,
		Type:          string(Debit),
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})
	if err != nil {
		return err
	}
	// Create credit entry
	err = s.ledgerRepo.CreateEntryWithTx(ctx, tx, database.Entry{
		ID:            uuid.New(),
		AccountID:     creditAccountID,
		TransactionID: transactionID,
		Amount:        amount,
		Type:          string(Credit),
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	})
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
