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
