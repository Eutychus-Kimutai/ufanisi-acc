package investment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}
type Worker struct {
	db           *sql.DB
	ledger       *domain.LedgerService
	repo         *repository.InvestmentRepository
	paymentRepo  *repository.PaymentsRepository
	outboxRepo   *repository.OutboxRepository
	channel      Publisher
	cfg          *rabbitmq.RabbitConfig
	capitalAccID uuid.UUID
}

// NewWorker creates an investment payment worker and resolves its capital account.
func NewWorker(db *sql.DB, channel Publisher, cfg *rabbitmq.RabbitConfig, queries *database.Queries) (*Worker, error) {
	capitalAccID, err := repository.NewRepository(db).GetCapitalAccount(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get capital account: %v", err)
	}

	return &Worker{
		db:           db,
		repo:         repository.NewInvestmentRepository(db),
		paymentRepo:  repository.NewPaymentsRepository(db),
		ledger:       domain.NewLedgerService(db, repository.NewRepository(db), repository.NewClientRepository(queries)),
		outboxRepo:   repository.NewOutboxRepository(db),
		channel:      channel,
		cfg:          cfg,
		capitalAccID: capitalAccID,
	}, nil
}

// HandlePaymentEvent resolves an investment payment and records its outcome.
func (w *Worker) HandlePaymentEvent(ctx context.Context, event commands.ResolvePaymentPayload) error {
	if event.Amount <= 0 {
		return errors.New("payment amount must be greater than zero")
	}
	txn, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	_, err = w.resolveInvestment(ctx, event, txn)
	if err != nil {
		fmt.Printf("Failed to resolve investment: %v\n", err)
		cmd, cmdErr := commands.NewCommand(
			commands.UnresolvedPayment,
			commands.UnresolvedPaymentPayload{
				Amount:     event.Amount,
				ExternalId: event.ExternalId,
				Reason:     err.Error(),
			},
		)
		if cmdErr != nil {
			return fmt.Errorf("failed to create unresolved payment command: %v", cmdErr)
		}
		cmdBytes, marshalErr := json.Marshal(cmd)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal unresolved payment command: %v", marshalErr)
		}

		outboxMsg := database.OutboxMessage{
			ID:            uuid.New(),
			AggregateType: "investment",
			AggregateID:   event.PaymentID,
			CommandType:   string(commands.UnresolvedPayment),
			Payload:       cmdBytes,
		}
		err = w.outboxRepo.CreateOutboxMessage(ctx, outboxMsg)
		if err != nil {
			return fmt.Errorf("failed to create outbox message: %v", err)
		}
		if commitErr := txn.Commit(); commitErr != nil {
			return fmt.Errorf("failed to commit transaction: %v", commitErr)
		}

		return err
	}
	//log.Printf("Successfully resolved investment: %+v\n", i)
	cmd, err := commands.NewCommand(
		commands.PaymentResolved,
		commands.PaymentResolvedPayload{
			ResolvedAt:     time.Now(),
			IdempotencyKey: event.ExternalId,
		},
	)
	if err != nil {
		return err
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}
	outboxMsg := database.OutboxMessage{
		ID:            uuid.New(),
		AggregateType: "investment",
		AggregateID:   event.PaymentID,
		CommandType:   string(commands.PaymentResolved),
		Payload:       cmdBytes,
	}
	err = w.outboxRepo.WithTx(txn).CreateOutboxMessage(ctx, outboxMsg)
	if err != nil {
		return fmt.Errorf("failed to create outbox message: %v", err)
	}

	if err := txn.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}

// resolveInvestment applies a payment to an investment within tx.
func (w *Worker) resolveInvestment(ctx context.Context, event commands.ResolvePaymentPayload, tx *sql.Tx) (*database.Investment, error) {

	accountRef := event.PaymentRef
	account, err := w.ledger.GetAccountDetails(ctx, accountRef)
	if err != nil {
		fmt.Printf("Failed to get account: %v", err)
		return nil, err
	}
	if account.Type != "investment" {
		return nil, errors.New("account is not of type investment")
	}
	// check if the investment already exists for this payment reference
	existingInv, err := w.repo.GetInvestmentByReference(ctx, accountRef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing investment: %v", err)
	}
	if existingInv == nil {

		// Save the investment to the database
		inv := database.Investment{
			Reference:        event.PaymentRef,
			PrincipalInitial: event.Amount,
			NextAccrualAt:    time.Now().AddDate(0, 1, 0), // set next accrual date to one month from now
			ClientID:         account.ClientID,
		}
		createdInv, err := w.repo.WithTx(tx).CreateInvestment(ctx, inv)
		if err != nil {
			return nil, err
		}
		// Post a ledger transaction to record the investment deposit
		ledgerTx := domain.Transaction{
			Id:         uuid.New(),
			Type:       "investment_deposit",
			ExternalId: event.ExternalId,
			Entries: []domain.Entry{
				{
					AccountId:  w.capitalAccID,
					Amount:     event.Amount,
					ExternalId: event.ExternalId,
					Type:       domain.Debit,
				},
				{
					AccountId:  account.ID,
					Amount:     event.Amount,
					ExternalId: event.ExternalId,
					Type:       domain.Credit,
				},
			},
		}
		log.Printf("posting ledger transaction: %+v\n", ledgerTx)
		err = w.ledger.WithTx(tx).PostTransaction(ctx, ledgerTx)
		if err != nil {
			return nil, fmt.Errorf("failed to post ledger transaction: %v", err)
		}

		return createdInv, nil
	}
	// Update the existing investment with the new amount
	updatedInv, err := w.repo.WithTx(tx).UpdateInvestmentPrincipal(ctx, database.Investment{
		PrincipalCurrent: existingInv.PrincipalCurrent + event.Amount,
		ID:               existingInv.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update investment: %v", err)
	}
	log.Printf("updated existing investment: %+v\n", event)
	ledgerTx := domain.Transaction{
		Id:         uuid.New(),
		Type:       "investment_deposit",
		ExternalId: event.ExternalId,
		Entries: []domain.Entry{
			{
				AccountId:  w.capitalAccID,
				Amount:     event.Amount,
				ExternalId: event.ExternalId,
				Type:       domain.Debit,
			},
			{
				AccountId:  account.ID,
				Amount:     event.Amount,
				ExternalId: event.ExternalId,
				Type:       domain.Credit,
			},
		},
	}
	err = w.ledger.WithTx(tx).PostTransaction(ctx, ledgerTx)
	if err != nil {
		return nil, fmt.Errorf("failed to post ledger transaction: %v", err)
	}
	return updatedInv, nil
}

func (w *Worker) RequestWithdrawal(ctx context.Context, invID uuid.UUID, amount int64, noticeMontrhs int32) error {
	if amount <= 0 {
		return errors.New("withdrawal amount must be greater than zero")
	}
	// validate the investment exists and is active
	inv, err := w.repo.GetInvestmentByID(ctx, invID)
	if err != nil {
		return fmt.Errorf("failed to get investment: %v", err)
	}
	if inv.Status != "active" {
		return errors.New("investment is not active")
	}
	if amount > inv.PrincipalCurrent {
		return errors.New("withdrawal amount exceeds current principal")
	}

	// aaaaacreate withdrawal record in database
	withdrawal := database.WithdrawalsPayable{
		InvestmentID:       inv.ID,
		Amount:             amount,
		NoticePeriodMonths: noticeMontrhs,
		RequestedAt:        time.Now(),
		Status:             "pending",
	}
	_, err = w.repo.CreateInvestmentWithdrawal(ctx, withdrawal)
	if err != nil {
		return fmt.Errorf("failed to create investment withdrawal record: %v", err)
	}

	// generate withdrawal notice
	err = w.GenerateWithdrawalNotice(inv, amount)
	if err != nil {
		return fmt.Errorf("failed to generate withdrawal notice: %v", err)
	}

	return nil
}

// ProcessEligibleWithdrawals completes withdrawals whose notice period has elapsed.
func (w *Worker) ProcessEligibleWithdrawals(ctx context.Context) error {
	withdrawals, err := w.repo.ListEligibleWithdrawals(ctx)
	if err != nil {
		return fmt.Errorf("failed to list eligible withdrawals: %v", err)
	}

	for _, wdr := range withdrawals {
		tx, err := w.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for withdrawal ID %v: %v", wdr.ID, err)
		}
		defer tx.Rollback()
		err = w.repo.UpdateWithdrawalStatusTx(ctx, tx, wdr.ID, "eligible")
		if err != nil {
			return fmt.Errorf("failed to update withdrawal status for withdrawal ID %v: %v", wdr.ID, err)
		}

		// generate withdrawal notice
		err = w.GenerateWithdrawalNotice(&database.Investment{ID: wdr.InvestmentID}, wdr.Amount)
		if err != nil {
			fmt.Printf("Failed to generate withdrawal notice for withdrawal ID %v: %v\n", wdr.ID, err)
			continue
		}

		// transfer funds (principal + accrued interest) to client account

		err = w.ledger.WithTx(tx).Transfer(ctx, w.capitalAccID, wdr.InvestmentID, wdr.Amount, "withdrawal_approved")
		if err != nil {
			fmt.Printf("Failed to transfer funds for withdrawal ID %v: %v\n", wdr.ID, err)
			continue
		}
		// update withdrawal status to processed
		err = w.repo.UpdateWithdrawalStatus(ctx, wdr.ID, "processed")
		if err != nil {
			fmt.Printf("Failed to update withdrawal status for withdrawal ID %v: %v\n", wdr.ID, err)
			continue

		}

		// generate withdrawal processed notice
		err = w.GenerateWithdrawalProcessedNotice(&database.Investment{ID: wdr.InvestmentID}, wdr.Amount)
		if err != nil {
			fmt.Printf("Failed to generate withdrawal processed notice for withdrawal ID %v: %v\n", wdr.ID, err)
			continue
		}
		tx.Commit()
	}
	return nil
}

func StartScheduler(ctx context.Context, worker *Worker, accrualWorker *AccrualWorker) error {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if ctx.Err() != nil {
				fmt.Println("Scheduler stopping...")
				return nil
			}
			fmt.Println("Running scheduled task: ProcessEligibleWithdrawals")
			err := worker.ProcessEligibleWithdrawals(ctx)
			if err != nil {
				fmt.Printf("error processing eligible withdrawals: %v\n", err)
			}
			err = accrualWorker.ProcessDueAccruals(ctx)
			if err != nil {
				fmt.Printf("error processing due accruals: %v\n", err)
			}
		case <-ctx.Done():
			fmt.Println("Scheduler stopping...")
			return nil
		}
	}
}
