package repository

import (
	"context"
	"database/sql"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type LoanRepository struct {
	db *database.Queries
}

func NewLoanRepository(db *sql.DB) *LoanRepository {

	return &LoanRepository{db: database.New(db)}
}

func (r *LoanRepository) GetLoanByReference(ctx context.Context, reference string) (database.Loan, error) {
	loan, err := r.db.GetLoanByReference(ctx, reference)
	if err != nil {
		return database.Loan{}, err
	}
	return loan, nil
}

func (r *LoanRepository) GetLoanByLoanNumber(ctx context.Context, loanNumber string) (database.Loan, error) {
	loan, err := r.db.GetLoanByLoanNumber(ctx, loanNumber)
	if err != nil {
		return database.Loan{}, err
	}
	return loan, nil
}

func (r *LoanRepository) GetLoanByID(ctx context.Context, id uuid.UUID) (database.Loan, error) {
	loan, err := r.db.GetLoanByID(ctx, id)
	if err != nil {
		return database.Loan{}, err
	}
	return loan, nil
}

func (r *LoanRepository) GetLoansByClientID(ctx context.Context, clientID uuid.UUID) ([]database.Loan, error) {
	loan, err := r.db.GetLoansByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return loan, nil
}

func (r *LoanRepository) UpdateLoanOutstandingAmount(ctx context.Context, id uuid.UUID, amount int64) error {
	err := r.db.UpdateLoanOutstandingAmount(ctx, database.UpdateLoanOutstandingAmountParams{
		ID:                id,
		OutstandingAmount: amount,
	})
	if err != nil {
		return err
	}
	return nil
}
