package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type LedgerRepository struct {
	db *database.Queries
}

var (
	ErrAccountNotFound = sql.ErrNoRows
	ErrAccountExists   = errors.New("account already exists")
)

// NewRepository creates a ledger repository backed by db.
func NewRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{
		db: database.New(db),
	}
}

// WithTx returns a ledger repository whose queries use tx.
func (l *LedgerRepository) WithTx(tx *sql.Tx) *LedgerRepository {
	return &LedgerRepository{db: l.db.WithTx(tx)}
}

func (l *LedgerRepository) CreateAccount(ctx context.Context, account database.Account) error {
	_, err := l.db.CreateAccount(ctx, database.CreateAccountParams{
		ID:   account.ID,
		Name: account.Name,
		Type: string(account.Type),
	})
	if err != nil {
		return err
	}
	return nil
}
func (l *LedgerRepository) GetAccountBalance(ctx context.Context, accountType string) (int64, error) {
	balance, err := l.db.GetAccountBalance(ctx, accountType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrAccountNotFound
		}
		return 0, err
	}
	return balance, nil
}

func (l *LedgerRepository) GetAccount(ctx context.Context, name string) (database.Account, error) {
	acc, err := l.db.GetAccount(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Account{}, ErrAccountNotFound
		}
		return database.Account{}, err
	}
	return database.Account{
		ID:   acc.ID,
		Name: acc.Name,
		Type: acc.Type,
	}, nil
}

func (l *LedgerRepository) GetAccountByID(ctx context.Context, accountId uuid.UUID) (database.Account, error) {
	acc, err := l.db.GetAccountByID(ctx, accountId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Account{}, ErrAccountNotFound
		}
		return database.Account{}, err
	}
	return acc, nil
}

// GetTransactionEntries returns all ledger entries for an account.
func (l *LedgerRepository) GetTransactionEntries(ctx context.Context, accountId uuid.UUID) ([]database.Entry, error) {
	tx, err := l.db.GetEntries(ctx, accountId)
	if err != nil {
		return nil, err
	}
	entries := make([]database.Entry, len(tx))
	for i, entry := range tx {
		entries[i] = database.Entry{
			TransactionID: entry.TransactionID,
			AccountID:     entry.AccountID,
			Amount:        entry.Amount,
			ExternalID:    entry.ExternalID,
			Type:          entry.Type,
		}
	}
	return entries, nil
}

func (l *LedgerRepository) CreateTransaction(ctx context.Context, transaction database.Transaction) error {
	_, err := l.db.CreateTransaction(ctx, database.CreateTransactionParams{
		ID:        transaction.ID,
		CreatedAt: transaction.CreatedAt,
		UpdatedAt: time.Now(),
		Type:      transaction.Type,
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *LedgerRepository) CreateEntry(ctx context.Context, entry database.CreateEntryParams) error {
	_, err := l.db.CreateEntry(ctx, database.CreateEntryParams{
		ID:            entry.ID,
		AccountID:     entry.AccountID,
		TransactionID: entry.TransactionID,
		Amount:        entry.Amount,
		Type:          entry.Type,
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *LedgerRepository) CreateUnresolvedPayment(ctx context.Context, payment database.UnresolvedPayment) error {
	err := l.db.CreateUnresolvedPayment(ctx, database.CreateUnresolvedPaymentParams{
		Reason:         payment.Reason,
		ClientRef:      payment.ClientRef,
		Amount:         payment.Amount,
		PaymentChannel: payment.PaymentChannel,
		ExternalID:     payment.ExternalID,
		RawEvent:       payment.RawEvent,
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *LedgerRepository) GetCapitalAccount(ctx context.Context) (uuid.UUID, error) {
	accID, err := l.db.GetCapitalAccount(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return accID, nil
}

// CreateEntryWithTx persists a ledger entry within tx.
func (l *LedgerRepository) CreateEntryWithTx(ctx context.Context, tx *sql.Tx, entry database.Entry) error {
	qtx := l.db.WithTx(tx)
	_, err := qtx.CreateEntry(ctx, database.CreateEntryParams{
		ID:            entry.ID,
		AccountID:     entry.AccountID,
		TransactionID: entry.TransactionID,
		ExternalID:    entry.ExternalID,
		Amount:        entry.Amount,
		Type:          entry.Type,
	})
	if err != nil {
		return err
	}
	return nil
}

// CreateTransactionWithTx persists a ledger transaction within tx.
func (l *LedgerRepository) CreateTransactionWithTx(ctx context.Context, tx *sql.Tx, transaction database.Transaction) error {
	qtx := l.db.WithTx(tx)
	_, err := qtx.CreateTransaction(ctx, database.CreateTransactionParams{
		ID:         transaction.ID,
		Type:       transaction.Type,
		CreatedAt:  transaction.CreatedAt,
		ExternalID: transaction.ExternalID,
		UpdatedAt:  time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}
