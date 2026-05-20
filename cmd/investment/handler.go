package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}
type Worker struct {
	db                 *sql.DB
	ledger             *domain.LedgerService
	repo               *repository.InvestmentRepository
	channel            Publisher
	cfg                *rabbitmq.RabbitConfig
	capitalAccID       uuid.UUID
	investorFundsAccID uuid.UUID
}

func NewWorker(db *sql.DB, channel Publisher, cfg *rabbitmq.RabbitConfig) (*Worker, error) {
	capitalAccID, err := repository.NewRepository(db).GetCapitalAccount(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get capital account: %v", err)
	}
	investorFundsAccID, err := repository.NewRepository(db).GetInvestorFundsAccount(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get investor funds account: %v", err)
	}
	return &Worker{
		db:                 db,
		repo:               repository.NewInvestmentRepository(db),
		ledger:             domain.NewLedgerService(db, repository.NewRepository(db)),
		channel:            channel,
		cfg:                cfg,
		capitalAccID:       capitalAccID,
		investorFundsAccID: investorFundsAccID,
	}, nil
}

func (w *Worker) HandlePaymentEvent(ctx context.Context, event payment.PaymentEvent) error {
	investment, client, err := w.resolveInvestment(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to resolve investment: %v", err)
	}

	rate, err := strconv.ParseFloat(investment.MonthlyRate, 64)
	if err != nil {
		return fmt.Errorf("failed to parse monthly rate: %v", err)
	}
	cmd, err := commands.NewCommand(
		commands.InvestmentCreated,
		commands.InvestmentCreatedPayload{
			Id:              investment.ID.String(),
			ClientId:        client.ID.String(),
			Principal:       investment.PrincipalCurrent,
			Status:          investment.Status,
			AccruedInterest: investment.AccruedInterest,
			NextAccrualDate: investment.NextAccrualAt.Format("2006-01-02"),
			MonthlyRate:     rate,
		},
	)
	if err != nil {
		return err
	}
	err = rabbitmq.PublishCommand(
		w.channel,
		w.cfg.Queues.Investment,
		cmd,
	)
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) resolveInvestment(ctx context.Context, event payment.PaymentEvent) (*database.Investment, *database.Client, error) {
	if event.ClientRef == "" {
		return nil, nil, errors.New("missing client reference in payment event")
	}
	if event.Destination != payment.DestinationAccountInvestment {
		return nil, nil, errors.New("invalid destination for investment payment")
	}

	accountRef := event.AccountReference
	account, err := w.ledger.GetAccount(ctx, accountRef)
	if err != nil {
		fmt.Printf("Failed to get account: %v", err)
		return nil, nil, err
	}
	if account.Type != "investment" {
		return nil, nil, errors.New("account is not of type investment")
	}
	parsedClientID, err := uuid.Parse(event.ClientRef)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid client reference: %v", err)
	}

	client, err := w.ledger.GetClient(ctx, parsedClientID)
	if err != nil {
		fmt.Printf("Failed to get client: %v", err)
		return nil, nil, err
	}
	// Save the investment to the database
	inv := database.Investment{
		ClientID:         client.ID,
		PrincipalInitial: event.Amount,
		PrincipalCurrent: event.Amount,
		AccruedInterest:  0,
	}
	createdInv, err := w.repo.CreateInvestment(ctx, inv)
	if err != nil {
		return nil, nil, err
	}
	tx := domain.Transaction{
		Id:   uuid.New(),
		Type: "investment_deposit",
		Entries: []domain.Entry{
			{
				AccountId: w.investorFundsAccID,
				Amount:    int64(event.Amount),
				Type:      "Credit",
			},
			{
				AccountId: account.ID,
				Amount:    int64(event.Amount),
				Type:      "Debit",
			},
		},
	}
	err = w.ledger.PostTransaction(ctx, tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to post ledger transaction: %v", err)
	}
	return createdInv, &client, nil
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

func (w *Worker) ProcessEligibleWithdrawals(ctx context.Context) error {
	withdrawals, err := w.repo.ListEligibleWithdrawals(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("failed to list eligible withdrawals: %v", err)
	}

	for _, wdr := range withdrawals {
		err := w.GenerateWithdrawalNotice(&database.Investment{ID: wdr.InvestmentID}, wdr.Amount)
		if err != nil {
			fmt.Printf("Failed to generate withdrawal notice for withdrawal ID %v: %v\n", wdr.ID, err)
			continue
		}
		// transfer funds (principal + accrued interest) to client account

		err = w.ledger.Transfer(ctx, w.capitalAccID, wdr.InvestmentID, wdr.Amount)
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
