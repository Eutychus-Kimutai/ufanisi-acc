package repository

import (
	"context"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

func (r *LedgerRepository) GetLoanByLoanNumber(ctx context.Context, loanNumber string) (database.Loan, error) {
	return r.db.GetLoanByLoanNumber(ctx, loanNumber)
}

func (r *LedgerRepository) GetLoanByID(ctx context.Context, id uuid.UUID) (database.Loan, error) {
	return r.db.GetLoanByID(ctx, id)
}

func (r *LedgerRepository) GetLoansByClientID(ctx context.Context, clientID uuid.UUID) ([]database.Loan, error) {
	return r.db.GetLoansByClientID(ctx, clientID)
}

func (r *LedgerRepository) UpdateLoanOutstandingAmount(ctx context.Context, id uuid.UUID, amount int64) error {
	return r.db.UpdateLoanOutstandingAmount(ctx, database.UpdateLoanOutstandingAmountParams{
		ID:                id,
		OutstandingAmount: amount,
	})
}
