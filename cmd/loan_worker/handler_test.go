package loanworker

import (
	"context"
	"database/sql"

	"testing"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
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
	setup := func(t *testing.T) (*testutils.MockChannel, *LoanWorker, payment.PaymentEvent, func()) {
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
			}{
				Unresolved:          "unresolved_payments",
				Loan:                "loans",
				Investment:          "investments",
				AccrualNotice:       "accrual_notices",
				InvestmentAccrued:   "investment_accrued",
				WithdrawalRequested: "withdrawal_requested",
				WithdrawalProcessed: "withdrawal_processed",
				MaturityNotice:      "maturity_notices",
			},
		})
		require.NoError(t, err)

		clientID := uuid.New()
		_, err = db.ExecContext(context.Background(), `INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', 'loan')`, clientID)
		require.NoError(t, err)

		loanID := uuid.New()
		_, err = db.ExecContext(context.Background(),
			`INSERT INTO loans (id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount) VALUES ($1, $2, 'LN123', 'Personal', 'active', 10000, 10000)`,
			loanID, clientID)
		require.NoError(t, err)

		event := payment.PaymentEvent{
			Amount:           5000,
			ExternalId:       "EXT123",
			Destination:      payment.DestinationAccount("loan"),
			PaymentChannel:   payment.PaymentChannel("mobile_money"),
			ClientRef:        clientID.String(),
			AccountReference: "LN123Mali",
		}
		cleanup := func() {
			defer dbCleanup()
			_, err = db.Exec("DELETE FROM loans WHERE id = $1", loanID)
			require.NoError(t, err)
			_, err = db.Exec("DELETE FROM clients WHERE id = $1", clientID)
			require.NoError(t, err)

		}

		return mockCh, worker, event, cleanup
	}
	t.Run("Test with invalid amount", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Amount = -100
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)

	})

	t.Run("Test invalid destination account", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Destination = payment.DestinationAccount("savings")
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("test missing external ID", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.ExternalId = ""
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("Test non-existent loan", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.AccountReference = "LN999Mali"
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("Test product type mismatch", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.AccountReference = "LN123Invalid"
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})

	t.Run("Test non-existent client", func(t *testing.T) {
		_, worker, event, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.ClientRef = uuid.New().String()
		err := worker.HandlePaymentEvent(context.Background(), event)
		require.Error(t, err)
	})
}
