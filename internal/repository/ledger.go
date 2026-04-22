package repository

import (
	"context"
	"database/sql"
	"errors"
	"log"
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

func NewRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{
		db: database.New(db),
	}
}
func NewDB(connStr string) (*sql.DB, *LedgerRepository, error) {
	openDb, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, nil, err
	}
	LedgerRepository := &LedgerRepository{
		db: database.New(openDb),
	}
	log.Printf("Connected to database: %s\n", connStr)
	log.Printf("LedgerRepository initialized: %+v\n", LedgerRepository)
	return openDb, LedgerRepository, nil
}

func (l *LedgerRepository) CreateAccount(ctx context.Context, account database.Account) error {
	acc, err := l.db.CreateAccount(ctx, database.CreateAccountParams{
		ID:   account.ID,
		Name: account.Name,
		Type: string(account.Type),
	})
	if err != nil {
		return err
	}
	log.Printf("Created account: %+v\n", acc)
	return nil
}

func (l *LedgerRepository) GetAccount(ctx context.Context, id uuid.UUID) (database.Account, error) {
	acc, err := l.db.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Account{}, ErrAccountNotFound
		}
		return database.Account{}, err
	}
	log.Printf("Retrieved account: %+v\n", acc)
	return acc, nil
}

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
			Type:          entry.Type,
		}
	}
	log.Printf("Retrieved entries for account %s: %+v\n", accountId, entries)
	return entries, nil
}

func (l *LedgerRepository) CreateTransaction(ctx context.Context, transaction database.Transaction) error {
	_, err := l.db.CreateTransaction(ctx, database.CreateTransactionParams{
		ID:        transaction.ID,
		Reference: transaction.Reference,
		CreatedAt: transaction.CreatedAt,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *LedgerRepository) CreateEntry(ctx context.Context, entry database.Entry) error {
	_, err := l.db.CreateEntry(ctx, database.CreateEntryParams{
		ID:            entry.ID,
		AccountID:     entry.AccountID,
		TransactionID: entry.TransactionID,
		Amount:        entry.Amount,
		Type:          entry.Type,
		CreatedAt:     entry.CreatedAt,
		UpdatedAt:     entry.UpdatedAt,
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *LedgerRepository) CreateUnresolvedPayment(ctx context.Context, payment database.UnresolvedPayment) error {
	_, err := l.db.CreateUnresolvedPayment(context.Background(), database.CreateUnresolvedPaymentParams{
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
