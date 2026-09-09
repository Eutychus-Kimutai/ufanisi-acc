package loan

import (
	"context"
	"database/sql"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"testing"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func setupDBAndCleanup(t *testing.T) (*sql.DB, func()) {
	db, err := testutils.SetupTestDB()
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, db.Close())
	}
	return db, cleanup
}
func TestWorker_HandlePaymentEvent(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*testutils.MockChannel, *LoanWorker, commands.ResolvePaymentPayload, uuid.UUID, func()) {
		mockCh := &testutils.MockChannel{}
		db, dbCleanup := setupDBAndCleanup(t)

		worker, err := NewWorker(db, mockCh, "payments.loan", &rabbitmq.RabbitConfig{
			Queues: struct {
				Loan                string `yaml:"loan"`
				Investment          string `yaml:"investment"`
				Unresolved          string `yaml:"unresolved"`
				AccrualNotice       string `yaml:"accrual_notice"`
				InvestmentAccrued   string `yaml:"investment_accrued"`
				WithdrawalRequested string `yaml:"withdrawal.requested"`
				WithdrawalProcessed string `yaml:"withdrawal.processed"`
				MaturityNotice      string `yaml:"maturity_notice"`
				Resolved            string `yaml:"resolved"`
			}{
				Loan:                "payments.loan",
				Investment:          "payments.investment",
				Unresolved:          "payments.unresolved",
				AccrualNotice:       "investments.accrual_notice",
				InvestmentAccrued:   "investments.accrued",
				WithdrawalRequested: "withdrawals.requested",
				WithdrawalProcessed: "withdrawals.processed",
				MaturityNotice:      "investments.maturity_notice",
				Resolved:            "payments.resolved",
			},
		})
		require.NoError(t, err)

		clientID := uuid.New()
		_, err = db.ExecContext(context.Background(), `INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', 'loan')`, clientID)
		require.NoError(t, err)
		accountID := uuid.New()
		_, err = db.ExecContext(context.Background(), `INSERT INTO accounts (id, name, client_id, type) VALUES ($1, $2, $3, 'loan')`, accountID, "LN123Mali", clientID)
		require.NoError(t, err)

		loanID := uuid.New()
		_, err = db.ExecContext(context.Background(),
			`INSERT INTO loans (id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount, reference) VALUES ($1, $2, 'LN123', 'Personal', 'active', 10000, 10000, 'LN123Mali')`,
			loanID, clientID)
		require.NoError(t, err)

		event := commands.ResolvePaymentPayload{
			Amount:     5000,
			ExternalId: "EXT123",
			PaymentRef: "LN123Mali",
		}
		cleanup := func() {
			defer dbCleanup()
			_, err = db.ExecContext(context.Background(), "DELETE FROM entries WHERE external_id = $1", event.ExternalId)
			require.NoError(t, err)
			_, err = db.ExecContext(context.Background(), "DELETE FROM transactions WHERE external_id = $1", event.ExternalId)
			require.NoError(t, err)
			_, err = db.ExecContext(context.Background(), "DELETE FROM loans WHERE id = $1", loanID)
			require.NoError(t, err)
			_, err = db.ExecContext(context.Background(), "DELETE FROM clients WHERE id = $1", clientID)
			require.NoError(t, err)
			_, err = db.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
			require.NoError(t, err)

		}

		return mockCh, worker, event, loanID, cleanup
	}
	t.Run("Test with invalid amount", func(t *testing.T) {
		_, worker, event, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Amount = -100
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)

	})

	t.Run("test missing external ID", func(t *testing.T) {
		_, worker, event, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.ExternalId = ""
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("Test non-existent loan", func(t *testing.T) {
		_, worker, event, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.PaymentRef = "LN999Mali"
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("Test inactive loan", func(t *testing.T) {
		_, worker, event, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		// Update the loan status to 'paid_off'
		_, err := worker.db.ExecContext(context.Background(), `UPDATE loans SET status = 'paid_off' WHERE reference = $1`, event.PaymentRef)
		require.NoError(t, err)

		err = worker.HandlePaymentEvent(context.Background(), event)
		require.NoError(t, err)
		loan, err := worker.loanRepo.GetLoanByReference(context.Background(), event.PaymentRef)
		require.NoError(t, err)
		require.Equal(t, "paid_off", loan.Status)

	})

	t.Run("Test overpayment", func(t *testing.T) {
		_, worker, event, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Amount = 15000
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.NoError(t, err)

		loan, err := worker.loanRepo.GetLoanByReference(context.Background(), event.PaymentRef)
		require.NoError(t, err)
		require.NotNil(t, loan)
		require.Equal(t, int64(0), loan.OutstandingAmount)

	})

}
