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
func (r *LoanRepository) CreateLoan(ctx context.Context, loan database.CreateLoanParams) (database.Loan, error) {
	createdLoan, err := r.db.CreateLoan(ctx, loan)
	if err != nil {
		return database.Loan{}, err
	}
	return createdLoan, nil
}

func (r *LoanRepository) GetLoanByReference(ctx context.Context, reference string) (*database.Loan, error) {
	loan, err := r.db.GetLoanByReference(ctx, reference)
	if err != nil {
		return &database.Loan{}, err
	}
	return &loan, nil
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

func (r *LoanRepository) CreateOverpayment(ctx context.Context, overpayment database.CreateOverpaymentParams) (database.Overpayment, error) {
	createdOverpayment, err := r.db.CreateOverpayment(ctx, overpayment)
	if err != nil {
		return database.Overpayment{}, err
	}
	return createdOverpayment, nil
}

func (r *LoanRepository) GetOverpaymentByLoanID(ctx context.Context, loanID uuid.UUID) (*database.Overpayment, error) {
	overpayments, err := r.db.GetOverpaymentByLoanID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	return &overpayments, nil
}

func (r *LoanRepository) UpdateOverpaymentAmount(ctx context.Context, id uuid.UUID, amount int64) error {
	err := r.db.UpdateOverpaymentAmount(ctx, database.UpdateOverpaymentAmountParams{
		ID:     id,
		Amount: amount,
	})
	if err != nil {
		return err
	}
	return nil
}
