package loanworker

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestWorker_HandlePaymentEvent(t *testing.T) {
	err := godotenv.Load("../../.env")
	require.NoError(t, err)
	db, err := sql.Open("postgres", os.Getenv("DB_URL"))
	require.NoError(t, err)
	defer db.Close()

	clientID := uuid.New()
	_, err = db.ExecContext(context.Background(), `INSERT INTO clients (id, name, client_type) VALUES ($1, 'Test Client', 'loan')`, clientID)
	require.NoError(t, err)

	loanID := uuid.New()
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO loans (id, client_id, loan_number, product_type, status, principal_amount, outstanding_amount) VALUES ($1, $2, 'LN123', 'Personal', 'active', 10000, 10000)`,
		loanID, clientID)
	require.NoError(t, err)

	defer db.Exec("DELETE FROM loans WHERE id = $1", loanID)
	defer db.Exec("DELETE FROM clients WHERE id = $1", clientID)

	mockCh := &MockChannel{}
	worker, err := NewWorker(db, mockCh, "payments.loan", &rabbitmq.RabbitConfig{
		Queues: struct {
			Loan       string `yaml:"loan"`
			Investment string `yaml:"investment"`
			Unresolved string `yaml:"unresolved"`
		}{
			Unresolved: "unresolved_payments",
			Loan:       "loans",
			Investment: "investments",
		},
	})
	require.NoError(t, err)

	event := payment.PaymentEvent{
		Amount:           5000,
		ExternalId:       "EXT123",
		Destination:      payment.DestinationAccount("loan"),
		PaymentChannel:   payment.PaymentChannel("mobile_money"),
		ClientRef:        clientID.String(),
		AccountReference: "LN123Mali",
	}
	err = worker.HandlePaymentEvent(context.Background(), event)
	require.NoError(t, err)

	//  Test invalid amount
	event.Amount = -100
	err = worker.HandlePaymentEvent(context.Background(), event)
	require.Error(t, err)

	// Test missing ExternalId
	event.Amount = 5000
	event.ExternalId = ""
	err = worker.HandlePaymentEvent(context.Background(), event)
	require.Error(t, err)

	// Test invalid destination
	event.ExternalId = "EXT123"
	event.Destination = payment.DestinationAccount("savings")
	err = worker.HandlePaymentEvent(context.Background(), event)
	require.Error(t, err)

	// Test non-existent loan
	event.Destination = payment.DestinationAccount("loan")
	event.AccountReference = "LN999Mali"
	err = worker.HandlePaymentEvent(context.Background(), event)
	t.Logf("Error for non-existent loan: %v", err)
	require.Error(t, err)

	// Test product type mismatch
	event.AccountReference = "LN123Invalid"
	err = worker.HandlePaymentEvent(context.Background(), event)
	t.Logf("Error for product type mismatch: %v", err)
	require.Error(t, err)

	// Test non-existent client
	event.AccountReference = "LN123Mali"
	event.ClientRef = uuid.New().String()
	err = worker.HandlePaymentEvent(context.Background(), event)
	t.Logf("Error for non-existent client: %v", err)
	require.Error(t, err)
}
