package loanworker

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
	amqp "github.com/rabbitmq/amqp091-go"
)

type publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

type LoanWorker struct {
	db        *sql.DB
	channel   publisher
	queuename string
	repo      repository.LedgerRepository
	cfg       *rabbitmq.RabbitConfig
}

func NewWorker(db *sql.DB, channel publisher, queuename string, cfg *rabbitmq.RabbitConfig) (*LoanWorker, error) {
	return &LoanWorker{
		db:        db,
		channel:   channel,
		queuename: queuename,
		repo:      *repository.NewRepository(db),
		cfg:       cfg,
	}, nil
}

func (w *LoanWorker) HandlePaymentEvent(ctx context.Context, event payment.PaymentEvent) error {
	if event.Amount <= 0 {
		return errors.New("Amount must be greater than zero")
	}
	if event.ExternalId == "" {
		return errors.New("ExternalId is required")
	}
	if event.Destination != payment.DestinationAccount("loan") {
		return errors.New("Invalid destination account")
	}

	loan, client, err := w.resolveLoan(ctx, event)
	if err != nil {
		return err
	}

	ref := fmt.Sprintf("loan_%s_%s_%s", loan.ID, event.PaymentChannel, event.ExternalId)

	cmd, err := commands.NewCommand(
		commands.ApplyLoanRepayment,
		commands.LoanRepaymentPayload{
			ClientID:       client.ID.String(),
			Amount:         event.Amount,
			PaymentChannel: string(event.PaymentChannel),
			ReferenceID:    event.ExternalId,
			Reference:      ref,
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to create command: %w", err)
	}
	err = rabbitmq.PublishCommand(
		w.channel,
		w.cfg.Queues.Unresolved,
		cmd,
	)
	if err != nil {
		return fmt.Errorf("Failed to publish command: %w", err)
	}
	return nil
}

func (w *LoanWorker) resolveLoan(ctx context.Context, event payment.PaymentEvent) (database.Loan, database.Client, error) {
	ref := w.getReference(event)

	parsedRef, err := payment.ParseAccountReference(ref)
	if err != nil {
		return database.Loan{}, database.Client{}, errors.New("Failed to parse account reference: " + err.Error())
	}
	loan, err := w.repo.GetLoanByLoanNumber(ctx, parsedRef.LoanNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, errors.New("Failed to retrieve loan")
		}
		return database.Loan{}, database.Client{}, fmt.Errorf("Failed to retrieve loan: %w", err)
	}

	if loan.ProductType != parsedRef.ProductType {
		return database.Loan{}, database.Client{}, errors.New("product type mismatch")
	}
	if loan.Status != "active" {
		return database.Loan{}, database.Client{}, errors.New("loan is not active")
	}

	client, err := w.repo.GetClientByID(ctx, event.ClientRef)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, errors.New("Failed to retrieve client")
		}

		err = w.repo.CreateUnresolvedPayment(ctx, database.UnresolvedPayment{
			ClientRef:      event.ClientRef,
			Amount:         event.Amount,
			PaymentChannel: string(event.PaymentChannel),
			ExternalID:     event.ExternalId,
			Reason:         "loan or client not found",
		})
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("Failed to create unresolved payment: %w", err)
		}

		cmd, err := commands.NewCommand(
			commands.UnresolvedPayment,
			commands.UnresolvedPaymentPayload{
				ClientRef:      event.ClientRef,
				Amount:         event.Amount,
				PaymentChannel: string(event.PaymentChannel),
				ExternalId:     event.ExternalId,
				Reason:         "loan or client not found",
			},
		)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("Failed to create command: %w", err)
		}

		err = w.channel.Publish("", w.cfg.Queues.Unresolved, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        cmd.Payload,
		})
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("Failed to publish unresolved payment: %w", err)
		}
		return database.Loan{}, database.Client{}, errors.New("loan or client not found - payment marked unresolved")
	}

	if client.ClientType != "loan" {
		return database.Loan{}, database.Client{}, errors.New("Client is not eligible for loan repayment")
	}

	loans, err := w.repo.GetLoansByClientID(ctx, client.ID)
	if err != nil || len(loans) == 0 {
		return database.Loan{}, database.Client{}, errors.New("no active loans found for client")
	}
	for _, l := range loans {
		if l.ID == loan.ID {
			return l, client, nil
		}
	}
	return database.Loan{}, database.Client{}, errors.New("loan does not belong to client")
}

func (w *LoanWorker) getReference(event payment.PaymentEvent) string {
	if event.AccountReference != "" {
		return event.AccountReference
	}
	return event.ClientRef
}
