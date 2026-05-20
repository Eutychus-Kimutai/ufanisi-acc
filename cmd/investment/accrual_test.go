package main

import (
	"database/sql"
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
		MonthlyRate:      "2.5",
		LastAccrualAt:    sql.NullTime{Time: time.Now().AddDate(0, -1, 0), Valid: true},
	}
	accrualWorker := NewAccrualWorker(nil, nil, nil)
	accrualAmount, err := accrualWorker.CalculateInvestmentAccrual(investment, 1)
	require.NoError(t, err)
	expectedAccrual := int64(2500)
	require.Equal(t, expectedAccrual, accrualAmount)

	// Test with multiple months
	accrualAmount, err = accrualWorker.CalculateInvestmentAccrual(investment, 3)
	require.NoError(t, err)
	expectedAccrual = int64(7500)
	require.Equal(t, expectedAccrual, accrualAmount)

	// Test with invalid rate
	investment.MonthlyRate = "invalid_rate"
	_, err = accrualWorker.CalculateInvestmentAccrual(investment, 30)
	require.Error(t, err)

	// Test with zero days
	investment.MonthlyRate = "0.30"
	accrualAmount, err = accrualWorker.CalculateInvestmentAccrual(investment, 0)
	require.NoError(t, err)
	require.Equal(t, int64(0), accrualAmount)
}
