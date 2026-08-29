package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
)

type Handler struct {
	paymentRepo *repository.PaymentsRepository
	accountRepo *repository.PaymentsRepository
	publisher   rabbitmq.Publisher
	db          *sql.DB
	cfg         *rabbitmq.RabbitConfig
}

// NewHandler creates a new Handler instance
func NewHandler(db *sql.DB, publisher rabbitmq.Publisher, cfg *rabbitmq.RabbitConfig) (*Handler, error) {
	paymentRepo := repository.NewPaymentsRepository(db)
	return &Handler{
		paymentRepo: paymentRepo,
		accountRepo: repository.NewPaymentsRepository(db),
		publisher:   publisher,
		db:          db,
		cfg:         cfg,
	}, nil
}

// HandlePayment processes a payment and publishes a message to RabbitMQ
func (h *Handler) HandlePayment(event payment.PaymentEvent) error {

	var ctx = context.Background()
	if event.Amount <= 0 {
		return errors.New("invalid payment amount")
	}
	if event.ExternalID == "" {
		return errors.New("missing external ID")
	}

	_, err := h.paymentRepo.GetPaymentByIdempotencyKey(ctx, event.ExternalID)
	if err == nil {
		return fmt.Errorf("payment with idempotency key %s already exists", event.ExternalID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check for existing payment: %v", err)
	}

	pmt, err := h.paymentRepo.CreatePayment(ctx, database.Payment{
		Amount:         event.Amount,
		PaymentType:    event.PaymentType,
		ExternalID:     event.ExternalID,
		IdempotencyKey: event.ExternalID,
		PaymentRef:     event.PaymentReference,
		Status:         "received",
		PhoneNumber:    event.PhoneNumber,
		RawEvent:       event.RawEvent,
	})
	if err != nil {
		return fmt.Errorf("failed to create payment: %v", err)
	}

	reference, err := h.paymentRepo.GetPaymentReference(ctx, event.PaymentReference)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// unresolved payment command to be published to RabbitMQ
			cmd, err := commands.NewCommand(
				commands.UnresolvedPayment,
				commands.UnresolvedPaymentPayload{
					Amount:     event.Amount,
					ExternalId: event.ExternalID,
					Reason:     "Payment reference not found in the system",
				},
			)
			if err != nil {
				return fmt.Errorf("failed to create command: %v", err)
			}

			// publish unresolved payment event to RabbitMQ
			err = rabbitmq.PublishCommand(h.publisher, h.cfg.Queues.Unresolved, cmd)

			if err != nil {
				return fmt.Errorf("failed to publish payment event: %v", err)
			}
			return nil
		} else {

			return fmt.Errorf("failed to get payment reference: %v", err)
		}
	}

	var queueName string
	switch reference.EntityType {
	case "loan":
		queueName = h.cfg.Queues.Loan
	case "investment":
		queueName = h.cfg.Queues.Investment
	default:
		return fmt.Errorf("unknown entity type: %s", reference.EntityType)

	}
	resolutionCmd, err := commands.NewCommand(
		commands.ResolvePayment,
		commands.ResolvePaymentPayload{
			PaymentID:   pmt.ID,
			PaymentRef:  event.PaymentReference,
			Amount:      event.Amount,
			ExternalId:  event.ExternalID,
			PhoneNumber: event.PhoneNumber,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create resolution command: %v", err)
	}

	err = rabbitmq.PublishCommand(h.publisher, queueName, resolutionCmd)
	if err != nil {
		return fmt.Errorf("failed to publish resolution command: %v", err)
	}
	err = h.paymentRepo.UpdatePaymentStatus(ctx, "resolving", pmt.ID)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %v", err)
	}

	return nil
}
