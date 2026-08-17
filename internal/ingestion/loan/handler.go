package loan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

var ErrLoanNotFound = errors.New("loan not found for the given payment reference")

type LoanWorker struct {
	db          *sql.DB
	channel     publisher
	queuename   string
	repo        repository.LedgerRepository
	loanRepo    *repository.LoanRepository
	accRepo     *repository.AccountsRepository
	paymentRepo *repository.PaymentsRepository
	cfg         *rabbitmq.RabbitConfig
}

func NewWorker(db *sql.DB, channel publisher, queuename string, cfg *rabbitmq.RabbitConfig) (*LoanWorker, error) {
	return &LoanWorker{
		db:          db,
		channel:     channel,
		queuename:   queuename,
		repo:        *repository.NewRepository(db),
		loanRepo:    repository.NewLoanRepository(db),
		accRepo:     repository.NewAccountsRepository(db),
		paymentRepo: repository.NewPaymentsRepository(db),
		cfg:         cfg,
	}, nil
}

func (w *LoanWorker) HandlePaymentEvent(ctx context.Context, event commands.ResolvePaymentPayload) error {
	log.Printf("Handling payment event: %+v\n", event)
	if event.Amount <= 0 {
		return errors.New("Amount must be greater than zero")
	}
	if event.ExternalId == "" {
		return errors.New("ExternalId is required")
	}
	_, _, err := w.resolveLoan(ctx, event)
	if err != nil {
		return err
	}

	cmd, err := commands.NewCommand(
		commands.PaymentResolved,
		commands.PaymentResolvedPayload{
			IdempotencyKey: event.ExternalId,
			ResolvedAt:     time.Now(),
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to create command: %w", err)
	}
	err = rabbitmq.PublishCommand(
		w.channel,
		w.cfg.Queues.Resolved,
		cmd,
	)
	if err != nil {
		return fmt.Errorf("Failed to publish command: %w", err)
	}
	return nil
}

func (w *LoanWorker) resolveLoan(ctx context.Context, event commands.ResolvePaymentPayload) (database.Loan, database.Client, error) {
	accDetails, err := w.accRepo.GetAccountDetails(ctx, event.PaymentRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, fmt.Errorf("account details not found for payment reference: %s", event.PaymentRef)

		}

		return database.Loan{}, database.Client{}, fmt.Errorf("failed to retrieve account details: %v", err)
	}
	log.Printf("Retrieved account details: %+v\n", accDetails)

	loan, err := w.loanRepo.GetLoanByReference(ctx, event.PaymentRef)
	if err != nil {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, errors.New("failed to retrieve loan by reference")
		} else {
			loan = nil

		}
		return database.Loan{}, database.Client{}, ErrLoanNotFound

	}
	if loan.Status == "paid_off" {
		return database.Loan{}, database.Client{}, errors.New("loan is already closed")
	}

	if loan.OutstandingAmount < event.Amount {
		overPayment := event.Amount - loan.OutstandingAmount
		op, err := w.loanRepo.GetOverpaymentByLoanID(ctx, loan.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to check for existing overpayment: %v", err)
		}
		if op != nil {
			err = w.loanRepo.UpdateOverpaymentAmount(ctx, op.ID, op.Amount+overPayment)
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to update overpayment amount: %v", err)
			}
		}
		_, err = w.loanRepo.CreateOverpayment(ctx, database.CreateOverpaymentParams{
			LoanID:     loan.ID,
			Amount:     overPayment,
			ExternalID: event.ExternalId,
		})
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to create overpayment record: %v", err)
		}
		err = w.loanRepo.UpdateLoanOutstandingAmount(ctx, loan.ID, 0)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to update loan outstanding amount: %v", err)
		}

	} else {

		outstandingAmount := loan.OutstandingAmount - event.Amount
		err = w.loanRepo.UpdateLoanOutstandingAmount(ctx, loan.ID, outstandingAmount)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to update loan outstanding amount: %v", err)
		}
	}

	return database.Loan{}, database.Client{}, nil
}
