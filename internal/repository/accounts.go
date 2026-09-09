package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
)

type AccountsRepository struct {
	db *database.Queries
}

// NewAccountsRepository creates an account repository backed by db.
func NewAccountsRepository(db *sql.DB) *AccountsRepository {
	return &AccountsRepository{
		db: database.New(db),
	}
}

// WithTx returns an account repository whose queries use tx.
func (a *AccountsRepository) WithTx(tx *sql.Tx) *AccountsRepository {
	return &AccountsRepository{db: a.db.WithTx(tx)}
}

func (a *AccountsRepository) CreateAccount(ctx context.Context, account database.Account) error {
	_, err := a.db.CreateAccount(ctx, database.CreateAccountParams{
		ID:   account.ID,
		Name: account.Name,
		Type: string(account.Type),
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *AccountsRepository) GetAccount(ctx context.Context, name string) (database.Account, error) {
	acc, err := a.db.GetAccount(ctx, name)
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

func (a *AccountsRepository) GetAccountDetails(ctx context.Context, name string) (database.GetAccountDetailsRow, error) {
	acc, err := a.db.GetAccountDetails(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.GetAccountDetailsRow{}, ErrAccountNotFound
		}
		return database.GetAccountDetailsRow{}, err
	}
	return database.GetAccountDetailsRow{
		ID:       acc.ID,
		Name:     acc.Name,
		Type:     acc.Type,
		ClientID: acc.ClientID,
	}, nil
}
