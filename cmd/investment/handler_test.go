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
	defer db.Close()
	// You would set up a test investment account, create a payment event, and then call HandlePaymentEvent.
	loanClientID := uuid.New()
	_, err = db.ExecContext(context.Background(), `INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', 'investment')`, loanClientID)
	if err != nil {
		t.Fatalf("Failed to insert test client: %v", err)
	}
	defer db.Exec("DELETE FROM clients WHERE id = $1", loanClientID)

	accountId := uuid.New()
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO accounts (id, name, type) VALUES ($1, 'Test Investment Account', 'investment')`,
		accountId)
	if err != nil {
		t.Fatalf("Failed to insert test account: %v", err)
	}
	defer db.Exec("DELETE FROM accounts WHERE id = $1", accountId)

	// Setup investor capital account
	capitalAccId := uuid.New()
	err = db.QueryRowContext(context.Background(),
		`SELECT id FROM accounts WHERE name = 'Investor Capital Account'`).Scan(&capitalAccId)
	if err != nil {
		t.Fatalf("Failed to fetch investor capital account: %v", err)
	}
	defer db.Exec("DELETE FROM accounts WHERE id = $1", capitalAccId)

	// Finally, you would check that the investment account was updated correctly and that any expected messages were published.
	event := payment.PaymentEvent{
		Amount:           1000,
		ExternalId:       "INVEXT123",
		PaymentChannel:   "mobile_money",
		AccountReference: accountId.String(),
		Destination:      "investment",
		ClientRef:        loanClientID.String(),
		PhoneNumber:      "0712345678",
	}
	// You would then call worker.HandlePaymentEvent(context.Background(), event) and check the results.
	// Mock channel
	mockCh := &testutils.MockChannel{}
	worker, err := NewWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: struct {
			Loan             string `yaml:"loan"`
			Investment       string `yaml:"investment"`
			Unresolved       string `yaml:"unresolved"`
			AccrualNotice    string `yaml:"accrual_notice"`
			WithdrawalNotice string `yaml:"withdrawal_notice"`
			MaturityNotice   string `yaml:"maturity_notice"`
		}{
			Unresolved:       "unresolved_payments",
			Loan:             "loans",
			Investment:       "investments",
			AccrualNotice:    "accrual_notices",
			WithdrawalNotice: "withdrawal_notices",
			MaturityNotice:   "maturity_notices",
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
	require.Equal(t, 1, len(mockCh.PublishedMessages), "Expected exactly one message to be published")
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

		invRepo := repository.NewInvestmentRepository(db)
		inv, err := invRepo.GetInvestmentByID(context.Background(), dbInvId)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		assert.Equal(t, loanClientID, inv.ClientID, "Expected ClientID in database to match the test client ID")
		assert.Equal(t, int64(1000), inv.PrincipalInitial, "Expected PrincipalInitial in database to match the payment event amount")
		assert.Equal(t, "0.3000", inv.AnnualRate, "Expected AnnualRate in database to be 0.30")
		assert.NotEmpty(t, inv.NextAccrualAt, "Expected NextAccrualAt in database to be set")

		// Clean up channel messages for next tests
		mockCh.PublishedMessages = nil
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
	event.AccountReference = accountId.String()
	event.ClientRef = "not-a-uuid"
	err = worker.HandlePaymentEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("Expected error for invalid client reference, got nil")
	}
	require.Error(t, err)

	// Test accrual processing
	accrualWorker := NewAccrualWorker(db, mockCh, &rabbitmq.RabbitConfig{
		Queues: struct {
			Loan             string `yaml:"loan"`
			Investment       string `yaml:"investment"`
			Unresolved       string `yaml:"unresolved"`
			AccrualNotice    string `yaml:"accrual_notice"`
			WithdrawalNotice string `yaml:"withdrawal_notice"`
			MaturityNotice   string `yaml:"maturity_notice"`
		}{
			Unresolved:       "unresolved_payments",
			Loan:             "loans",
			Investment:       "investments",
			AccrualNotice:    "accrual_notices",
			WithdrawalNotice: "withdrawal_notices",
			MaturityNotice:   "maturity_notices",
		},
	})
	invId := uuid.New()
	invClientID := uuid.New()
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO clients (id, name, client_type) VALUES ($1, 'Accrual Test Client', 'investment')`, invClientID)
	if err != nil {
		t.Fatalf("Failed to insert test client: %v", err)
	}
	defer db.Exec("DELETE FROM clients WHERE id = $1", invClientID)
	investmentForAccrual := database.Investment{
		ID:               invId,
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
		invId, invClientID, int64(1000), int64(1000), "active", "0.3000", time.Now().AddDate(0, -1, 0), time.Now().AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("Failed to insert test investment: %v", err)
	}
	defer db.Exec("DELETE FROM investments WHERE id = $1", invId)
	err = accrualWorker.ProcessInvestmentAccrual(context.Background(), &investmentForAccrual)
	require.NoError(t, err, "Expected to create accrual worker without error")

	// Verify the investment accrual has correct values
	invRepo := repository.NewInvestmentRepository(db)
	inv, err := invRepo.GetInvestmentByID(context.Background(), invId)
	require.NoError(t, err, "Expected to retrieve investment from database without error")
	assert.Equal(t, int64(24), inv.AccruedInterest, "Expected AccruedInterest in database to be 24 after accrual processing")
	assert.WithinDuration(t, time.Now(), inv.LastAccrualAt.Time, time.Minute, "Expected LastAccrualAt to be updated to now")
	assert.WithinDuration(t, time.Now().AddDate(0, 1, 0), inv.NextAccrualAt, time.Minute, "Expected NextAccrualAt to be updated to one month from now")

	// Verify accrual notice message was published
	require.Equal(t, 1, len(mockCh.PublishedMessages), "Expected exactly one message to be published after accrual processing")
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

		assert.Equal(t, invId.String(), payload.InvestmentId, "Expected InvestmentId in published message to match the test investment ID")
		assert.Equal(t, int64(24), payload.AccrualAmount, "Expected AccrualAmount in published message to match the accrual")
		assert.Equal(t, int64(24), payload.NewAccruedTotal, "Expected NewAccruedTotal in published message to match the accrual")
		assert.NotEmpty(t, payload.NextAccrualDate, "Expected NextAccrualDate in published message to be set")
	}

}
