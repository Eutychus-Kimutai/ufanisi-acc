package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
)

type AccrualWorker struct {
	db         *sql.DB
	cfg        *rabbitmq.RabbitConfig
	channel    Publisher
	repo       *repository.InvestmentRepository
	outboxRepo *repository.OutboxRepository
}

func NewAccrualWorker(db *sql.DB, channel Publisher, cfg *rabbitmq.RabbitConfig) *AccrualWorker {
	return &AccrualWorker{
		db:         db,
		cfg:        cfg,
		channel:    channel,
		repo:       repository.NewInvestmentRepository(db),
		outboxRepo: repository.NewOutboxRepository(db),
	}
}

func (w *AccrualWorker) CalculateInvestmentAccrual(inv *database.Investment, months int) (int64, error) {
	parsedRate, err := strconv.ParseFloat(inv.MonthlyRate, 64)
	if err != nil {
		return 0, err
	}

	// Accrual is computed and rounded to the nearest cent in this function.
	accrualAmount := float64(inv.PrincipalInitial) * (parsedRate / 100) * float64(months)
	return int64(math.Round(accrualAmount)), nil
}

func (w *AccrualWorker) ProcessInvestmentAccrual(ctx context.Context, inv *database.Investment) error {
	// Calculate days since last accrual
	now := time.Now()
	monthsElapsed := 0
	previousAccrual := inv.LastAccrualAt.Time
	if inv.LastAccrualAt.Valid {
		previousAccrual = inv.LastAccrualAt.Time
	} else {
		previousAccrual = inv.NextAccrualAt.AddDate(0, -1, 0)
	}
	for {
		nextBoundary := previousAccrual.AddDate(0, monthsElapsed+1, 0)
		if nextBoundary.After(now) {
			break
		}
		monthsElapsed++
	}
	if monthsElapsed <= 0 {
		return nil
	}
	accrualAmount, err := w.CalculateInvestmentAccrual(inv, monthsElapsed)
	if err != nil {
		return fmt.Errorf("error calculating investment accrual: %s", err)
	}
	newAccruedTotal := inv.AccruedInterest + accrualAmount
	// next accrual in a month
	lastProcessed := previousAccrual.AddDate(0, monthsElapsed, 0)
	nextAccrualDate := lastProcessed.AddDate(0, 1, 0)

	// Update investment with new accrued interest and next accrual date

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	err = w.repo.UpdateInvestmentTx(ctx, tx, database.Investment{
		AccruedInterest:  newAccruedTotal,
		LastAccrualAt:    sql.NullTime{Time: lastProcessed, Valid: true},
		ID:               inv.ID,
		ClientID:         inv.ClientID,
		NextAccrualAt:    nextAccrualDate,
		UpdatedAt:        now,
		Status:           inv.Status,
		PrincipalCurrent: inv.PrincipalCurrent,
	})
	if err != nil {
		return fmt.Errorf("failed to update investment: %w", err)
	}
	cmd := commands.InvestmentAccruedPayload{
		InvestmentId:    inv.ID.String(),
		AccrualAmount:   accrualAmount,
		NewAccruedTotal: newAccruedTotal,
		NextAccrualDate: nextAccrualDate.String(),
	}
	command, err := commands.NewCommand(commands.InvestmentAccrued, cmd)
	if err != nil {
		return err
	}
	outboxMsg := database.OutboxMessage{
		AggregateID:   inv.ID,
		AggregateType: "investment",
		CommandType:   string(command.Type),
		Payload:       command.Payload,
		Status:        "pending",
	}

	err = w.outboxRepo.WithTx(tx).CreateOutboxMessage(ctx, outboxMsg)

	if err != nil {
		return fmt.Errorf("failed to create outbox message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (w *AccrualWorker) ProcessDueAccruals(ctx context.Context) error {
	dueAccruals, err := w.repo.ListDueForAccrual(ctx, time.Now())
	if err != nil {
		return err
	}
	for _, inv := range dueAccruals {
		if ctx.Err() != nil {
			// Stop processing if context is cancelled and return the error
			return ctx.Err()
		}
		// create a per item grace context for one investment.
		c := context.WithoutCancel(ctx)
		graceCtx, cancel := context.WithTimeout(c, 30*time.Second)
		err := w.ProcessInvestmentAccrual(graceCtx, &inv)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
