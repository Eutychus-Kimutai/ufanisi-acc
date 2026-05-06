package investment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}
type Worker struct {
	db           *sql.DB
	ledger       *domain.LedgerService
	repo         *repository.InvestmentRepository
	channel      publisher
	cfg          *rabbitmq.RabbitConfig
	capitalAccID uuid.UUID
}

func NewWorker(db *sql.DB, channel publisher, cfg *rabbitmq.RabbitConfig) (*Worker, error) {
	capitalAccID, err := repository.NewRepository(db).GetInvestorCapitalAccount(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get investor capital account: %v", err)
	}
	return &Worker{
		db:           db,
		repo:         repository.NewInvestmentRepository(db),
		ledger:       domain.NewLedgerService(db, repository.NewRepository(db)),
		channel:      channel,
		cfg:          cfg,
		capitalAccID: capitalAccID,
	}, nil
}

func (w *Worker) HandlePaymentEvent(ctx context.Context, event payment.PaymentEvent) error {
	investment, client, err := w.resolveInvestment(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to resolve investment: %v", err)
	}

	rate, err := strconv.ParseFloat(investment.AnnualRate, 64)
	if err != nil {
		return fmt.Errorf("failed to parse annual rate: %v", err)
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
			AnnualRate:      rate,
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
	client, err := w.ledger.GetClient(ctx, event.ClientRef)
	if err != nil {
		fmt.Printf("Failed to get client: %v", err)
		return nil, nil, err
	}
	// Save the investment to the database
	inv := database.Investment{
		ClientID:         client.ID,
		PrincipalInitial: event.Amount,
	}
	createdInv, err := w.repo.CreateInvestment(ctx, inv)
	if err != nil {
		return nil, nil, err
	}

	return createdInv, &client, nil
}
