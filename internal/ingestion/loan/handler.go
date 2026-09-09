package loan

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

var ErrLoanNotFound = errors.New("loan not found for the given payment reference")
var LoanAlreadyPaidOff = errors.New("loan is already paid off")

type LoanWorker struct {
	db          *sql.DB
	channel     Publisher
	queuename   string
	ledger      *domain.LedgerService
	repo        repository.LedgerRepository
	loanRepo    *repository.LoanRepository
	accRepo     *repository.AccountsRepository
	paymentRepo *repository.PaymentsRepository
	outboxRepo  *repository.OutboxRepository
	cfg         *rabbitmq.RabbitConfig
}

// NewWorker creates a loan payment worker and its repositories.
func NewWorker(db *sql.DB, channel Publisher, queuename string, cfg *rabbitmq.RabbitConfig) (*LoanWorker, error) {
	return &LoanWorker{
		db:          db,
		channel:     channel,
		queuename:   queuename,
		ledger:      domain.NewLedgerService(db, repository.NewRepository(db), repository.NewClientRepository(database.New(db))),
		repo:        *repository.NewRepository(db),
		loanRepo:    repository.NewLoanRepository(db),
		accRepo:     repository.NewAccountsRepository(db),
		paymentRepo: repository.NewPaymentsRepository(db),
		outboxRepo:  repository.NewOutboxRepository(db),
		cfg:         cfg,
	}, nil
}

func (w *LoanWorker) HandlePaymentEvent(ctx context.Context, event commands.ResolvePaymentPayload) error {
	if event.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if event.ExternalId == "" {
		return errors.New("externalId is required")

	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	_, _, err = w.resolveLoan(ctx, event, tx)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// resolveLoan applies a payment to a loan and records the result within tx.
func (w *LoanWorker) resolveLoan(ctx context.Context, event commands.ResolvePaymentPayload, tx *sql.Tx) (database.Loan, database.Client, error) {
	accDetails, err := w.accRepo.GetAccountDetails(ctx, event.PaymentRef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, fmt.Errorf("account details not found for payment reference: %s", event.PaymentRef)

		}

		return database.Loan{}, database.Client{}, fmt.Errorf("failed to retrieve account details: %v", err)
	}
	capitalAccID, err := w.repo.GetCapitalAccount(ctx)
	if err != nil {
		return database.Loan{}, database.Client{}, fmt.Errorf("failed to retrieve capital account ID: %v", err)
	}

	log.Printf("Retrieved account details: %+v\n", accDetails)

	loan, err := w.loanRepo.GetLoanByReference(ctx, event.PaymentRef)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return database.Loan{}, database.Client{}, errors.New("failed to retrieve loan by reference")
		}
		return database.Loan{}, database.Client{}, ErrLoanNotFound

	}
	log.Printf("Retrieved loan: %+v\n", loan)
	if loan.Status == "paid_off" {
		ov, err := w.loanRepo.GetOverpaymentByExternalId(ctx, event.ExternalId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("Error retrieving overpayment by external ID: %v\n", err)
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to retrieve overpayment by external ID: %v", err)
			} else {
				_, err = w.loanRepo.WithTx(tx).CreateOverpayment(ctx, database.CreateOverpaymentParams{
					LoanID:     loan.ID,
					Amount:     event.Amount,
					ExternalID: event.ExternalId,
				})
				if err != nil {
					return database.Loan{}, database.Client{}, fmt.Errorf("failed to create overpayment record for paid off loan: %v", err)
				}

				cmd, err := commands.NewCommand(commands.PaymentResolved, commands.PaymentResolvedPayload{
					IdempotencyKey: event.ExternalId,
					ResolvedAt:     time.Now(),
				})
				if err != nil {
					return database.Loan{}, database.Client{}, fmt.Errorf("failed to create resolved payment command: %v", err)
				}
				cmdBytes, err := json.Marshal(cmd)
				if err != nil {
					return database.Loan{}, database.Client{}, fmt.Errorf("failed to marshal resolved payment command: %v", err)
				}
				err = w.outboxRepo.WithTx(tx).CreateOutboxMessage(ctx, database.OutboxMessage{
					AggregateType: "loan",
					AggregateID:   loan.ID,
					CommandType:   string(cmd.Type),
					Payload:       json.RawMessage(cmdBytes),
				})
				if err != nil {
					return database.Loan{}, database.Client{}, fmt.Errorf("failed to create outbox message for unresolved payment: %v", err)
				}
				transaction := domain.Transaction{
					Id:         uuid.New(),
					Type:       "loan_overpayment",
					ExternalId: event.ExternalId,
					Entries: []domain.Entry{
						{
							AccountId:  capitalAccID,
							Type:       domain.Debit,
							ExternalId: event.ExternalId,
							Amount:     event.Amount,
						},
						{
							AccountId:  accDetails.ID,
							Type:       domain.Credit,
							ExternalId: event.ExternalId,
							Amount:     event.Amount,
						},
					},
				}
				err = w.ledger.WithTx(tx).PostTransaction(ctx, transaction)
				if err != nil {
					return database.Loan{}, database.Client{}, fmt.Errorf("failed to post ledger transaction for overpayment: %v", err)
				}

				return *loan, database.Client{}, nil
			}
		}
		if ov != nil {
			cmd, err := commands.NewCommand(commands.UnresolvedPayment, commands.UnresolvedPaymentPayload{
				Amount:     event.Amount,
				ClientRef:  loan.ClientID.String(),
				ExternalId: event.ExternalId,
				Reason:     fmt.Sprintf("Overpayment with external ID: %s already exists, skipping creation", event.ExternalId),
			})
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to create unresolved payment command for existing overpayment: %v", err)
			}
			cmdBytes, err := json.Marshal(cmd)
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to marshal unresolved payment command for existing overpayment: %v", err)
			}
			err = w.outboxRepo.WithTx(tx).CreateOutboxMessage(ctx, database.OutboxMessage{
				AggregateType: "loan",
				AggregateID:   loan.ID,
				CommandType:   string(cmd.Type),
				Payload:       json.RawMessage(cmdBytes),
			})
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to create outbox message for unresolved payment of existing overpayment: %v", err)
			}
			log.Printf("Created outbox message for unresolved payment of existing overpayment with external ID: %v\n", event.ExternalId)
			return database.Loan{}, database.Client{}, nil
		}

	}
	log.Printf("Loan status is not paid off, proceeding with payment resolution for loan ID %d\n", loan.ID)
	if loan.OutstandingAmount < event.Amount {
		overPayment := event.Amount - loan.OutstandingAmount

		ov, err := w.loanRepo.GetOverpaymentByExternalId(ctx, event.ExternalId)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to retrieve overpayment by external ID: %v", err)
			} else {
				ov = nil
			}
		}
		if ov == nil {
			_, err = w.loanRepo.WithTx(tx).CreateOverpayment(ctx, database.CreateOverpaymentParams{
				LoanID:     loan.ID,
				Amount:     overPayment,
				ExternalID: event.ExternalId,
			})

			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to create overpayment record: %v", err)
			}
			err = w.loanRepo.WithTx(tx).UpdateLoanOutstandingAmount(ctx, loan.ID, 0)
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to update loan outstanding amount: %v", err)
			}
			err = w.loanRepo.WithTx(tx).UpdateLoanStatus(ctx, loan.ID, "paid_off")
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to update loan status: %v", err)
			}
			cmd, err := commands.NewCommand(commands.PaymentResolved, commands.PaymentResolvedPayload{
				IdempotencyKey: event.ExternalId,
				ResolvedAt:     time.Now(),
			})
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to create resolved payment command: %v", err)
			}
			cmdBytes, err := json.Marshal(cmd)
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to marshal resolved payment command: %v", err)
			}
			err = w.outboxRepo.WithTx(tx).CreateOutboxMessage(ctx, database.OutboxMessage{
				AggregateType: "loan",
				AggregateID:   loan.ID,
				CommandType:   string(cmd.Type),
				Payload:       json.RawMessage(cmdBytes),
				Status:        "pending",
			})
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to create outbox message for resolved payment: %v", err)
			}
			transaction := domain.Transaction{
				Id:         uuid.New(),
				Type:       "loan_payment",
				ExternalId: event.ExternalId,
				Entries: []domain.Entry{
					{
						AccountId:  capitalAccID,
						Type:       domain.Debit,
						ExternalId: event.ExternalId,
						Amount:     event.Amount,
					},
					{
						AccountId:  accDetails.ID,
						Type:       domain.Credit,
						ExternalId: event.ExternalId,
						Amount:     event.Amount,
					},
				},
			}
			err = w.ledger.WithTx(tx).PostTransaction(ctx, transaction)
			if err != nil {
				return database.Loan{}, database.Client{}, fmt.Errorf("failed to post ledger transaction: %v", err)
			}

		} else {
			return database.Loan{}, database.Client{}, fmt.Errorf("overpayment with external ID: %v already exists", event.ExternalId)
		}

	} else {
		err = w.loanRepo.WithTx(tx).UpdateLoanOutstandingAmount(ctx, loan.ID, loan.OutstandingAmount-event.Amount)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to update loan outstanding amount: %v", err)
		}
		cmd, err := commands.NewCommand(commands.PaymentResolved, commands.PaymentResolvedPayload{
			IdempotencyKey: event.ExternalId,
			ResolvedAt:     time.Now(),
		})
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to create resolved payment command: %v", err)
		}
		transaction := domain.Transaction{
			Id:         uuid.New(),
			Type:       "loan_payment",
			ExternalId: event.ExternalId,
			Entries: []domain.Entry{
				{
					AccountId:  capitalAccID,
					Type:       domain.Debit,
					ExternalId: event.ExternalId,
					Amount:     event.Amount,
				},
				{
					AccountId:  accDetails.ID,
					Type:       domain.Credit,
					ExternalId: event.ExternalId,
					Amount:     event.Amount,
				},
			},
		}
		log.Printf("Transaction details: %+v\n", transaction.Entries)
		err = w.ledger.WithTx(tx).PostTransaction(ctx, transaction)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to post ledger transaction: %v", err)
		}

		cmdBytes, err := json.Marshal(cmd)
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to marshal resolved payment command: %v", err)
		}
		err = w.outboxRepo.WithTx(tx).CreateOutboxMessage(ctx, database.OutboxMessage{
			AggregateType: "loan",
			AggregateID:   loan.ID,
			CommandType:   string(cmd.Type),
			Payload:       json.RawMessage(cmdBytes),
			Status:        "pending",
		})
		if err != nil {
			return database.Loan{}, database.Client{}, fmt.Errorf("failed to create outbox message for resolved payment: %v", err)
		}

	}
	return *loan, database.Client{}, nil
}
