package investment

import (
	"context"
	"database/sql"
	"testing"
	"time"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildQueuesConfig() rabbitmq.QueueConfig {
	return rabbitmq.QueueConfig{
		Unresolved:          "unresolved_payments",
		Loan:                "loans",
		Investment:          "investments",
		AccrualNotice:       "accrual_notices",
		InvestmentAccrued:   "investment_accrued",
		WithdrawalRequested: "withdrawal_requested",
		WithdrawalProcessed: "withdrawal_processed",
		MaturityNotice:      "maturity_notices",
	}
}

func SetupDBWithCleanup(t *testing.T) (*sql.DB, func()) {
	db, err := testutils.SetupTestDB()
	require.NoError(t, err)

	cleanup := func() {
		assert.NoError(t, db.Close())
	}

	return db, cleanup
}

func createTestAccount(t *testing.T, db *sql.DB, clientId uuid.UUID, accountName string) uuid.UUID {
	accountID := uuid.New()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO accounts (id, name, client_id, type) VALUES ($1, $2, $3, 'investment')`, accountID, accountName, clientId)
	require.NoError(t, err, "Failed to insert test account")
	return accountID
}

func createTestClient(t *testing.T, db *sql.DB, clientType string) uuid.UUID {
	clientID := uuid.New()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', $2)`, clientID, clientType)
	require.NoError(t, err, "Failed to insert test client")
	return clientID
}

func seedInvestmentForAccrual(t *testing.T, db *sql.DB, clientID uuid.UUID) uuid.UUID {
	invID := uuid.New()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO investments (id, reference, client_id, principal_initial, principal_current, status, monthly_rate, last_accrual_at, next_accrual_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		invID, "INVACC-2026", clientID, int64(1000), int64(1000), "active", "2.5000", time.Now().AddDate(0, -1, 0), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err, "Failed to insert test investment")
	return invID
}

func newValidPaymentEvent(clientID uuid.UUID) commands.ResolvePaymentPayload {
	return commands.ResolvePaymentPayload{
		Amount:      1000,
		PaymentRef:  "Test-2026",
		ExternalId:  "EXT123",
		PhoneNumber: "0712345678",
	}
}

func BuildWorkerAndDispatcher(t *testing.T, db *sql.DB, mockCh *testutils.MockChannel, queries *database.Queries) (*Worker, *OutboxDispatcher, *AccrualWorker) {
	queues := buildQueuesConfig()
	worker, err := NewWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: queues,
	}, queries)
	require.NoError(t, err, "Failed to create worker")
	dispatcher := &OutboxDispatcher{
		repo:    repository.NewOutboxRepository(db),
		channel: mockCh,
		cfg: &rabbitmq.RabbitConfig{
			Queues: queues,
		},
		locker: "investment_dispatcher",
	}
	accrualWorker := NewAccrualWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: queues,
	})

	return worker, dispatcher, accrualWorker
}

func TestHandlePaymentEvent(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*testutils.MockChannel, *Worker, commands.ResolvePaymentPayload, *AccrualWorker, *OutboxDispatcher, func()) {
		db, dbCleanup := SetupDBWithCleanup(t)
		mockCh := &testutils.MockChannel{}

		invClientID := createTestClient(t, db, "investment")
		accountID := createTestAccount(t, db, invClientID, "Test-2026")
		loanClientID := createTestClient(t, db, "loan")
		invID := seedInvestmentForAccrual(t, db, invClientID)
		var createdInvestmentID uuid.UUID
		worker, dispatcher, accrualWorker := BuildWorkerAndDispatcher(t, db, mockCh, database.New(db))

		event := newValidPaymentEvent(invClientID)

		ctx := context.Background()

		cleanup := func() {
			defer dbCleanup()
			_, err := db.ExecContext(ctx, `DELETE FROM entries WHERE account_id = $1`, accountID)
			require.NoError(t, err, "Failed to clean up entries for test account")
			if worker != nil {
				_, err = db.ExecContext(ctx, `DELETE FROM entries WHERE account_id = $1`, worker.capitalAccID)
				require.NoError(t, err, "Failed to clean up entries for investor funds account")
			}
			_, err = db.ExecContext(ctx, `DELETE FROM transactions`)
			require.NoError(t, err)

			if createdInvestmentID != uuid.Nil {
				_, err = db.ExecContext(ctx, `DELETE FROM investments WHERE id = $1`, createdInvestmentID)
				require.NoError(t, err)
			}

			_, err = db.ExecContext(ctx, `DELETE FROM investments WHERE id = $1`, invID)
			require.NoError(t, err)

			_, err = db.ExecContext(ctx, `DELETE FROM clients WHERE id IN ($1, $2)`, loanClientID, invClientID)
			require.NoError(t, err)

			_, err = db.ExecContext(ctx, `DELETE FROM accounts WHERE id = $1`, accountID)
			require.NoError(t, err)

		}
		return mockCh, worker, event, accrualWorker, dispatcher, cleanup
	}

	t.Run("Test with invalid amount", func(t *testing.T) {
		_, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Amount = -100

		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.Error(t, err, "Expected error for invalid amount")
	})

	t.Run("Test db persistence", func(t *testing.T) {
		_, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.NoError(t, err, "Expected no error for valid payment event")

		// Verify the investment was created in the database with correct values
		invRepo := repository.NewInvestmentRepository(worker.db)
		inv, err := invRepo.GetInvestmentByReference(context.Background(), event.PaymentRef)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		assert.Equal(t, event.Amount, inv.PrincipalInitial, "Expected PrincipalInitial in database to match the payment event amount")
		assert.Equal(t, "2.5000", inv.MonthlyRate, "Expected MonthlyRate in database to be 2.5")
		assert.NotEmpty(t, inv.NextAccrualAt, "Expected NextAccrualAt in database to be set")
	})

	t.Run("Test ledger balances", func(t *testing.T) {
		_, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.NoError(t, err, "Expected no error for valid payment event")

		// Test ledger balances after processing the payment event
		debitBalance, err := worker.ledger.GetBalance(context.Background(), "debit")
		if err != nil {
			t.Fatalf("Failed to get debit balance: %v", err)
		}
		creditBalance, err := worker.ledger.GetBalance(context.Background(), "credit")
		if err != nil {
			t.Fatalf("Failed to get credit balance: %v", err)
		}
		if debitBalance != creditBalance {
			t.Fatalf("Ledger balances do not match: debit=%d, credit=%d", debitBalance, creditBalance)
		}
	})
}
