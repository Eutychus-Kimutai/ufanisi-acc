package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlePaymentEvent(t *testing.T) {
	// This test would be similar to the one in loan_worker, but adapted for investments.
	db, err := testutils.SetupTestDB()
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	var worker *Worker
	var dispatcher *OutboxDispatcher
	// Setup test account and client
	accountID := uuid.New()
	loanClientID := uuid.New()
	invClientID := uuid.New()
	invID := uuid.New()
	accountName := "Test Investment Account"
	var createdInvestmentID uuid.UUID

	// clean up function
	ctx := context.Background()
	t.Cleanup(func() {
		_, err = db.ExecContext(ctx, `DELETE FROM entries WHERE account_id = $1`, accountID)
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

		require.NoError(t, db.Close())
	})
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO accounts (id, name, type) VALUES ($1, $2, 'investment')`, accountID, accountName+accountID.String())
	require.NoError(t, err, "Failed to insert test account")
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', 'investment')`, loanClientID)
	require.NoError(t, err, "Failed to insert test client")

	// Finally, you would check that the investment account was updated correctly and that any expected messages were published.
	event := payment.PaymentEvent{
		Amount:           1000,
		ExternalId:       "INVEXT123",
		PaymentChannel:   "mobile_money",
		AccountReference: accountID.String(),
		Destination:      "investment",
		ClientRef:        loanClientID.String(),
		PhoneNumber:      "0712345678",
	}
	// You would then call worker.HandlePaymentEvent(context.Background(), event) and check the results.
	// Mock channel
	mockCh := &testutils.MockChannel{}
	worker, err = NewWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: struct {
			Loan              string `yaml:"loan"`
			Investment        string `yaml:"investment"`
			Unresolved        string `yaml:"unresolved"`
			AccrualNotice     string `yaml:"accrual_notice"`
			InvestmentAccrued string `yaml:"investment_accrued"`
			WithdrawalNotice  string `yaml:"withdrawal_notice"`
			MaturityNotice    string `yaml:"maturity_notice"`
		}{
			Unresolved:        "unresolved_payments",
			Loan:              "loans",
			Investment:        "investments",
			AccrualNotice:     "accrual_notices",
			InvestmentAccrued: "investment_accrued",
			WithdrawalNotice:  "withdrawal_notices",
			MaturityNotice:    "maturity_notices",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Check published messages before handling the event
	if len(mockCh.PublishedMessages) != 0 {
		t.Fatalf("Expected no published messages before handling event, got %d", len(mockCh.PublishedMessages))
	}

	// Test queue is correct
	if worker.cfg.Queues.Investment != "investments" {
		t.Fatalf("Expected investment queue to be 'investments', got '%s'", worker.cfg.Queues.Investment)
	}
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandlePaymentEvent failed: %v", err)
	}
	// Setup dispatcher
	dispatcher = &OutboxDispatcher{
		repo:    repository.NewOutboxRepository(db),
		channel: mockCh,
		cfg: &rabbitmq.RabbitConfig{
			Queues: struct {
				Loan              string `yaml:"loan"`
				Investment        string `yaml:"investment"`
				Unresolved        string `yaml:"unresolved"`
				AccrualNotice     string `yaml:"accrual_notice"`
				InvestmentAccrued string `yaml:"investment_accrued"`
				WithdrawalNotice  string `yaml:"withdrawal_notice"`
				MaturityNotice    string `yaml:"maturity_notice"`
			}{
				Unresolved:        "unresolved_payments",
				Loan:              "loans",
				Investment:        "investments",
				AccrualNotice:     "accrual_notices",
				InvestmentAccrued: "investment_accrued",
				WithdrawalNotice:  "withdrawal_notices",
				MaturityNotice:    "maturity_notices",
			},
		},
		locker: "investment_dispatcher",
	}

	err = dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err, "Expected DispatchOnce to complete without error")
	type Msg struct {
		Payload struct {
			ClientId        string  `json:"client_id"`
			Principal       int64   `json:"principal"`
			AnnualRate      float64 `json:"annual_rate"`
			Status          string  `json:"status"`
			AccruedInterest float64 `json:"accrued_interest"`
			NextAccrualDate string  `json:"next_accrual_date"`
			Id              string  `json:"id"`
		} `json:"payload"`
	}
	for _, msg := range mockCh.PublishedMessages {
		assert.Equal(t, worker.cfg.Queues.Investment, msg.Queue, "Expected message to be published to the correct investment queue")
		var payload Msg
		err := json.Unmarshal(msg.Payload, &payload)
		if err != nil {
			t.Fatalf("Failed to unmarshal published message payload: %v", err)
		}
		assert.Equal(t, loanClientID.String(), payload.Payload.ClientId, "Expected ClientId in published message to match the test client ID")
		assert.Equal(t, int64(1000), payload.Payload.Principal, "Expected Principal in published message to match the payment event amount")
		assert.Equal(t, 0.30, payload.Payload.AnnualRate, "Expected AnnualRate in published message to be 0.30")
		assert.Equal(t, "active", payload.Payload.Status, "Expected Status in published message to be 'active'")
		assert.Equal(t, float64(0), payload.Payload.AccruedInterest, "Expected AccruedInterest in published message to be 0 for a new investment")
		assert.NotEmpty(t, payload.Payload.NextAccrualDate, "Expected NextAccrualDate in published message to be set")
		assert.NotEmpty(t, payload.Payload.Id, "Expected Id in published message to be set")

		// Verify the investment was created in the database with correct values
		dbInvId, err := uuid.Parse(payload.Payload.Id)
		require.NoError(t, err, "Expected Id in published message to be a valid UUID")
		createdInvestmentID = dbInvId

		invRepo := repository.NewInvestmentRepository(db)
		inv, err := invRepo.GetInvestmentByID(context.Background(), dbInvId)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		assert.Equal(t, loanClientID, inv.ClientID, "Expected ClientID in database to match the test client ID")
		assert.Equal(t, int64(1000), inv.PrincipalInitial, "Expected PrincipalInitial in database to match the payment event amount")
		assert.Equal(t, "0.3000", inv.AnnualRate, "Expected AnnualRate in database to be 0.30")
		assert.NotEmpty(t, inv.NextAccrualAt, "Expected NextAccrualAt in database to be set")

		// Test ledger balances after processing the payment event
		ledgerBalance, err := worker.ledger.GetBalance(context.Background(), accountID)
		require.NoError(t, err, "Expected to get ledger balance without error")
		expextedBalance := float64(1000) // For an investment, the account balance should increase.
		assert.Equal(t, expextedBalance, ledgerBalance, "Expected account balance to be updated correctly after processing payment event")
		// Clean up channel messages for next tests
		mockCh.PublishedMessages = nil
		// clean up outbox messages
		_, err = db.ExecContext(ctx, `DELETE FROM outbox_messages`)
		require.NoError(t, err, "Failed to clean up outbox messages after test")
	}

	// Test invalid destination
	event.Destination = "invalid_destination"
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("Expected error for invalid destination, got nil")
	}
	require.Error(t, err)

	// Test missing client reference
	event.Destination = "investment"
	event.ClientRef = ""
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("Expected error for missing client reference, got nil")
	}
	require.Error(t, err)

	// Test accountref not uuid
	event.ClientRef = loanClientID.String()
	event.AccountReference = "not-a-uuid"
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("Expected error for invalid account reference, got nil")
	}
	require.Error(t, err)

	// Test clientref not uuid
	event.AccountReference = accountID.String()
	event.ClientRef = "not-a-uuid"
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("Expected error for invalid client reference, got nil")
	}
	require.Error(t, err)

	// Test accrual processing
	accrualWorker := NewAccrualWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: struct {
			Loan              string `yaml:"loan"`
			Investment        string `yaml:"investment"`
			Unresolved        string `yaml:"unresolved"`
			AccrualNotice     string `yaml:"accrual_notice"`
			InvestmentAccrued string `yaml:"investment_accrued"`
			WithdrawalNotice  string `yaml:"withdrawal_notice"`
			MaturityNotice    string `yaml:"maturity_notice"`
		}{
			Unresolved:        "unresolved_payments",
			Loan:              "loans",
			Investment:        "investments",
			AccrualNotice:     "accrual_notices",
			InvestmentAccrued: "investment_accrued",
			WithdrawalNotice:  "withdrawal_notices",
			MaturityNotice:    "maturity_notices",
		},
	})
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO clients (id, name, client_type) VALUES ($1, 'Accrual Test Client', 'investment')`, invClientID)
	if err != nil {
		t.Fatalf("Failed to insert test client: %v", err)
	}
	defer db.Exec("DELETE FROM clients WHERE id = $1", invClientID)
	investmentForAccrual := database.Investment{
		ID:               invID,
		ClientID:         invClientID,
		PrincipalInitial: int64(1000),
		PrincipalCurrent: int64(1000),
		Status:           "active",
		AnnualRate:       "0.3000",
		LastAccrualAt:    sql.NullTime{Time: time.Now().AddDate(0, -1, 0), Valid: true},
		NextAccrualAt:    time.Now().AddDate(0, 1, 0),
	}
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO investments (id, client_id, principal_initial, principal_current, status, annual_rate, last_accrual_at, next_accrual_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		invID, invClientID, int64(1000), int64(1000), "active", "0.3000", time.Now().AddDate(0, -1, 0), time.Now().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("Failed to insert test investment: %v", err)
	}
	defer db.Exec("DELETE FROM investments WHERE id = $1", invID)
	err = accrualWorker.ProcessInvestmentAccrual(context.Background(), &investmentForAccrual)
	require.NoError(t, err, "Expected to create accrual worker without error")

	// Verify the investment accrual has correct values
	invRepo := repository.NewInvestmentRepository(db)
	inv, err := invRepo.GetInvestmentByID(context.Background(), invID)
	require.NoError(t, err, "Expected to retrieve investment from database without error")
	assert.Equal(t, int64(25), inv.AccruedInterest, "Expected AccruedInterest in database to be 25 after accrual processing")
	assert.WithinDuration(t, time.Now(), inv.LastAccrualAt.Time, time.Minute, "Expected LastAccrualAt to be updated to now")
	assert.WithinDuration(t, time.Now().AddDate(0, 1, 0), inv.NextAccrualAt, time.Minute, "Expected NextAccrualAt to be updated to one month from now")

	var (
		outboxCount         int
		outboxStatus        string
		outboxCommandType   string
		outboxAggregateID   string
		outboxAggregateType string
		outboxPayload       []byte
	)
	err = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*), status, command_type, aggregate_id, aggregate_type, payload FROM outbox_messages WHERE aggregate_id = $1 GROUP BY status, command_type, aggregate_id, aggregate_type, payload`,
		invID.String()).Scan(&outboxCount, &outboxStatus, &outboxCommandType, &outboxAggregateID, &outboxAggregateType, &outboxPayload)
	require.NoError(t, err, "Expected to query outbox messages without error")
	assert.Equal(t, "pending", outboxStatus)
	assert.Equal(t, "INVESTMENT_ACCRUED", outboxCommandType)
	assert.Equal(t, invID.String(), outboxAggregateID)
	assert.Equal(t, "investment", outboxAggregateType)
	assert.NotEmpty(t, outboxPayload, "Expected outbox payload to be set")

	// Process the outbox messages
	err = dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err, "Expected DispatchOnce to complete without error")
	require.Equal(t, 1, len(mockCh.PublishedMessages), "Expected exactly one message to be published")
	type AccrualNoticeMsg struct {
		InvestmentId    string `json:"investment_id"`
		AccrualAmount   int64  `json:"accrual_amount"`
		NewAccruedTotal int64  `json:"new_accrued_total"`
		NextAccrualDate string `json:"next_accrual_date"`
	}
	for _, msg := range mockCh.PublishedMessages {
		assert.Equal(t, accrualWorker.cfg.Queues.AccrualNotice, msg.Queue, "Expected message to be published to the correct accrual notice queue")

		var payload AccrualNoticeMsg
		err := json.Unmarshal(msg.Payload, &payload)
		require.NoError(t, err, "Expected to unmarshal published message payload without error")

		assert.Equal(t, invID.String(), payload.InvestmentId, "Expected InvestmentId in published message to match the test investment ID")
		assert.Equal(t, int64(25), payload.AccrualAmount, "Expected AccrualAmount in published message to match the accrual")
		assert.Equal(t, int64(25), payload.NewAccruedTotal, "Expected NewAccruedTotal in published message to match the accrual")
		assert.NotEmpty(t, payload.NextAccrualDate, "Expected NextAccrualDate in published message to be set")
	}

}
