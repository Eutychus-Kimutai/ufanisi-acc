package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
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

func createTestAccount(t *testing.T, db *sql.DB, accountName string) uuid.UUID {
	accountID := uuid.New()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO accounts (id, name, type) VALUES ($1, $2, 'investment')`, accountID, accountName+accountID.String())
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
		`INSERT INTO investments (id, client_id, principal_initial, principal_current, status, monthly_rate, last_accrual_at, next_accrual_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		invID, clientID, int64(1000), int64(1000), "active", "2.5000", time.Now().AddDate(0, -1, 0), time.Now().AddDate(0, 1, 0))
	require.NoError(t, err, "Failed to insert test investment")
	return invID
}

func newValidPaymentEvent(accountID, clientID uuid.UUID) payment.PaymentEvent {
	return payment.PaymentEvent{
		Amount:           1000,
		ExternalId:       "INVEXT123",
		PaymentChannel:   "mobile_money",
		AccountReference: accountID.String(),
		Destination:      "investment",
		ClientRef:        clientID.String(),
		PhoneNumber:      "0712345678",
	}
}

func BuildWorkerAndDispatcher(t *testing.T, db *sql.DB, mockCh *testutils.MockChannel) (*Worker, *OutboxDispatcher, *AccrualWorker) {
	queues := buildQueuesConfig()
	worker, err := NewWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: queues,
	})
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
	setup := func(t *testing.T) (*testutils.MockChannel, *Worker, payment.PaymentEvent, *AccrualWorker, *OutboxDispatcher, func()) {
		db, cleanup := SetupDBWithCleanup(t)
		mockCh := &testutils.MockChannel{}
		accountID := createTestAccount(t, db, "Test Investment Account")
		loanClientID := createTestClient(t, db, "loan")
		invClientID := createTestClient(t, db, "investment")
		invID := seedInvestmentForAccrual(t, db, invClientID)
		var createdInvestmentID uuid.UUID
		worker, dispatcher, accrualWorker := BuildWorkerAndDispatcher(t, db, mockCh)

		event := newValidPaymentEvent(accountID, loanClientID)

		ctx := context.Background()

		cleanup = func() {
			_, err := db.ExecContext(ctx, `DELETE FROM entries WHERE account_id = $1`, accountID)
			require.NoError(t, err, "Failed to clean up entries for test account")
			if worker != nil {
				_, err = db.ExecContext(ctx, `DELETE FROM entries WHERE account_id = $1`, worker.investorFundsAccID)
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

	t.Run("Test invalid destination account", func(t *testing.T) {
		_, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		event.Destination = payment.DestinationAccount("savings")
		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.Error(t, err, "Expected error for invalid destination account")
	})

	t.Run("test handle payment with published message", func(t *testing.T) {
		mockCh, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)

		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.NoError(t, err, "Expected no error for valid payment event")
		assert.Equal(t, 1, len(mockCh.PublishedMessages), "Expected one message to be published")
		publishedMsg := mockCh.PublishedMessages[0]
		assert.Equal(t, worker.cfg.Queues.Investment, publishedMsg.Queue, "Expected message to be published to the correct queue")
	})

	t.Run("Verify published messages correctness and db persistence", func(t *testing.T) {
		mockCh, worker, event, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		err := worker.HandlePaymentEvent(context.Background(), event)
		assert.NoError(t, err, "Expected no error for valid payment event")
		assert.Equal(t, 1, len(mockCh.PublishedMessages), "Expected one message to be published")
		publishedMsg := mockCh.PublishedMessages[0]
		assert.Equal(t, worker.cfg.Queues.Investment, publishedMsg.Queue, "Expected message to be published to the correct queue")
		type cmd struct {
			CommandType string                            `json:"command_type"`
			Payload     commands.InvestmentCreatedPayload `json:"payload"`
		}
		wrapper := cmd{}
		err = json.Unmarshal(publishedMsg.Payload, &wrapper)
		assert.NoError(t, err, "Expected to unmarshal published message without error")

		payload := wrapper.Payload
		assert.NoError(t, err, "Expected to unmarshal published message payload without error")
		assert.Equal(t, event.ClientRef, payload.ClientId, "Expected ClientId in published message to match the test client ID")
		assert.Equal(t, event.Amount, payload.Principal, "Expected Principal in published message to match the payment event amount")
		assert.Equal(t, 2.5, payload.MonthlyRate, "Expected MonthlyRate in published message to be 2.5")
		assert.Equal(t, "active", payload.Status, "Expected Status in published message to be 'active'")
		assert.Equal(t, int64(0), payload.AccruedInterest, "Expected AccruedInterest in published message to be 0 for a new investment")
		assert.NotEmpty(t, payload.NextAccrualDate, "Expected NextAccrualDate in published message to be set")
		assert.NotEmpty(t, payload.Id, "Expected Id in published message to be set")

		// Verify the investment was created in the database with correct values
		dbInvId, err := uuid.Parse(payload.Id)
		require.NoError(t, err, "Expected Id in published message to be a valid UUID")

		invRepo := repository.NewInvestmentRepository(worker.db)
		inv, err := invRepo.GetInvestmentByID(context.Background(), dbInvId)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		assert.Equal(t, event.ClientRef, inv.ClientID.String(), "Expected ClientID in database to match the test client ID")
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
		ledgerBalance, err := worker.ledger.GetBalance(context.Background(), uuid.MustParse(event.AccountReference))
		require.NoError(t, err, "Expected to get ledger balance without error")
		expectedBalance := float64(1000)
		assert.Equal(t, expectedBalance, ledgerBalance, "Expected account balance to be updated correctly after processing payment event")
	})
}
