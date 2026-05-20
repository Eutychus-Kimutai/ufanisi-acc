package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type InvestmentRepository struct {
	db *database.Queries
}

func NewInvestmentRepository(db *sql.DB) *InvestmentRepository {
	return &InvestmentRepository{db: database.New(db)}
}

func (r *InvestmentRepository) WithTx(tx *sql.Tx) *InvestmentRepository {
	return &InvestmentRepository{db: r.db.WithTx(tx)}
}

func (r *InvestmentRepository) CreateInvestment(ctx context.Context, inv database.Investment) (*database.Investment, error) {
	inv.NextAccrualAt = time.Now().AddDate(0, 1, 0)
	createdInv, err := r.db.CreateInvestment(ctx, database.CreateInvestmentParams{
		ClientID:         inv.ClientID,
		PrincipalInitial: inv.PrincipalInitial,
		NextAccrualAt:    inv.NextAccrualAt,
	})
	if err != nil {
		return nil, err
	}
	return &createdInv, nil
}

func (r *InvestmentRepository) GetInvestmentByID(ctx context.Context, id uuid.UUID) (*database.Investment, error) {
	inv, err := r.db.GetInvestmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvestmentRepository) GetInvestmentsCapitalAccount(ctx context.Context) (*uuid.UUID, error) {
	capitalAccID, err := r.db.GetInvestorCapitalAccount(ctx)
	if err != nil {
		return nil, err
	}
	return &capitalAccID, nil
}

func (r *InvestmentRepository) UpdateInvestment(ctx context.Context, inv database.Investment) error {
	err := r.db.UpdateInvestment(ctx, database.UpdateInvestmentParams{
		ID:               inv.ID,
		PrincipalCurrent: inv.PrincipalCurrent,
		AccruedInterest:  inv.AccruedInterest,
		Status:           inv.Status,
		NextAccrualAt:    inv.NextAccrualAt,
		LastAccrualAt:    inv.LastAccrualAt,
		ClientID:         inv.ClientID,
		UpdatedAt:        time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *InvestmentRepository) ListDueForAccrual(ctx context.Context, currentTime time.Time) ([]database.Investment, error) {
	invs, err := r.db.GetDueAccruals(ctx, currentTime)
	if err != nil {
		return nil, err
	}
	return invs, nil
}

func (r *InvestmentRepository) CreateAccrualRecord(ctx context.Context, record database.InvestmentAccrual) error {
	_, err := r.db.CreateAccrualRecord(ctx, database.CreateAccrualRecordParams{
		InvestmentID:     record.InvestmentID,
		Amount:           record.Amount,
		AccrualTimestamp: record.AccrualTimestamp,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *InvestmentRepository) CreateInvestmentWithdrawal(ctx context.Context, withdrawal database.InvestmentWithdrawal) (*database.InvestmentWithdrawal, error) {
	eligibleAt := withdrawal.RequestedAt.AddDate(0, int(withdrawal.NoticePeriodMonths), 0)
	createdWithdrawal, err := r.db.CreateInvestmentWithdrawal(ctx, database.CreateInvestmentWithdrawalParams{
		InvestmentID:       withdrawal.InvestmentID,
		Amount:             withdrawal.Amount,
		NoticePeriodMonths: withdrawal.NoticePeriodMonths,
		RequestedAt:        withdrawal.RequestedAt,
		EligibleAt:         eligibleAt,
		Status:             withdrawal.Status,
	})
	if err != nil {
		return nil, err
	}
	return &createdWithdrawal, nil
}

func (r *InvestmentRepository) ListEligibleWithdrawals(ctx context.Context, currentTime time.Time) ([]database.InvestmentWithdrawal, error) {
	withdrawals, err := r.db.ListEligibleWithdrawals(ctx)
	if err != nil {
		return nil, err
	}
	return withdrawals, nil
}

// wrap the updateinvestment so partial updates can be done in a transaction
func (r *InvestmentRepository) UpdateInvestmentTx(ctx context.Context, tx *sql.Tx, inv database.Investment) error {
	return r.WithTx(tx).UpdateInvestment(ctx, database.Investment{
		ID:               inv.ID,
		ClientID:         inv.ClientID,
		NextAccrualAt:    inv.NextAccrualAt,
		LastAccrualAt:    inv.LastAccrualAt,
		UpdatedAt:        time.Now(),
		Status:           inv.Status,
		PrincipalCurrent: inv.PrincipalCurrent,
		AccruedInterest:  inv.AccruedInterest,
	})
}
