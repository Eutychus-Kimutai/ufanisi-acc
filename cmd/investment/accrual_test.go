package main

import (
	"database/sql"
	"math"
	"testing"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAccrual(t *testing.T) {
	investment := &database.Investment{
		ID:               uuid.New(),
		ClientID:         uuid.New(),
		PrincipalInitial: 100000,
		AnnualRate:       "0.30",
		LastAccrualAt:    sql.NullTime{Time: time.Now().AddDate(0, -1, 0), Valid: true},
	}
	accrualWorker := NewAccrualWorker(nil, nil, nil)
	accrualAmount, err := accrualWorker.CalculateInvestmentAccrual(investment, 30)
	require.NoError(t, err)
	expectedAccrual := math.Floor(float64(100000 * 0.30 * 30 / 365))
	require.Equal(t, expectedAccrual, float64(accrualAmount))

	// Test with invalid rate
	investment.AnnualRate = "invalid_rate"
	_, err = accrualWorker.CalculateInvestmentAccrual(investment, 30)
	require.Error(t, err)

	// Test with zero days
	investment.AnnualRate = "0.30"
	accrualAmount, err = accrualWorker.CalculateInvestmentAccrual(investment, 0)
	require.NoError(t, err)
	require.Equal(t, int64(0), accrualAmount)
}
