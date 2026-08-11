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
		Reference:        inv.Reference,
		ClientID:         inv.ClientID,
		PrincipalInitial: inv.PrincipalInitial,
		NextAccrualAt:    inv.NextAccrualAt,
	})
	if err != nil {
		return nil, err
	}
	return &createdInv, nil
}
func (r *InvestmentRepository) GetInvestmentByReference(ctx context.Context, reference string) (*database.Investment, error) {
	inv, err := r.db.GetInvestmentByReference(ctx, reference)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvestmentRepository) GetInvestmentByID(ctx context.Context, id uuid.UUID) (*database.Investment, error) {
	inv, err := r.db.GetInvestmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvestmentRepository) GetCapitalAccount(ctx context.Context) (*uuid.UUID, error) {
	capitalAccID, err := r.db.GetCapitalAccount(ctx)
	if err != nil {
		return nil, err
	}
	return &capitalAccID, nil
}

func (r *InvestmentRepository) UpdateInvestment(ctx context.Context, inv database.Investment) error {
	err := r.db.UpdateInvestment(ctx, database.UpdateInvestmentParams{
		PrincipalCurrent: inv.PrincipalCurrent,
		Status:           inv.Status,
		UpdatedAt:        time.Now(),
		ID:               inv.ID,
	})
	if err != nil {
		return err
	}
	return nil
}
func (r *InvestmentRepository) UpdateInvestmentPrincipal(ctx context.Context, inv database.Investment) (*database.Investment, error) {
	inv, err := r.db.UpdateInvestmentPrincipal(ctx, database.UpdateInvestmentPrincipalParams{
		PrincipalCurrent: inv.PrincipalCurrent,
		ID:               inv.ID,
	})
	if err != nil {
		return &database.Investment{}, err
	}
	return &inv, nil
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

func (r *InvestmentRepository) CreateInvestmentWithdrawal(ctx context.Context, withdrawal database.WithdrawalsPayable) (*database.WithdrawalsPayable, error) {
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

func (r *InvestmentRepository) ListEligibleWithdrawals(ctx context.Context) ([]database.WithdrawalsPayable, error) {
	withdrawals, err := r.db.ListEligibleWithdrawals(ctx)
	if err != nil {
		return nil, err
	}
	return withdrawals, nil
}

// wrap the updateinvestment so partial updates can be done in a transaction
func (r *InvestmentRepository) UpdateInvestmentTx(ctx context.Context, tx *sql.Tx, inv database.Investment) error {
	return r.WithTx(tx).db.UpdateInvestmentAccrual(ctx, database.UpdateInvestmentAccrualParams{
		ID:              inv.ID,
		NextAccrualAt:   inv.NextAccrualAt,
		LastAccrualAt:   inv.LastAccrualAt,
		UpdatedAt:       time.Now(),
		AccruedInterest: inv.AccruedInterest,
	})
}

// update withdrawal status
func (r *InvestmentRepository) UpdateWithdrawalStatus(ctx context.Context, withdrawalID uuid.UUID, newStatus string) error {
	err := r.db.UpdateWithdrawalStatus(ctx, database.UpdateWithdrawalStatusParams{
		Status: newStatus,
		ID:     withdrawalID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *InvestmentRepository) UpdateWithdrawalStatusTx(ctx context.Context, tx *sql.Tx, withdrawalID uuid.UUID, newStatus string) error {
	err := r.WithTx(tx).UpdateWithdrawalStatus(ctx, withdrawalID, newStatus)
	if err != nil {
		return err
	}
	return nil
}
