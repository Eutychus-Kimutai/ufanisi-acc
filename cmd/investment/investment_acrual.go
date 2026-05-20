package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type AccrualWorker struct {
	db      *sql.DB
	cfg     *rabbitmq.RabbitConfig
	channel Publisher
	repo    *repository.InvestmentRepository
}

func NewAccrualWorker(db *sql.DB, channel Publisher, cfg *rabbitmq.RabbitConfig) *AccrualWorker {
	return &AccrualWorker{
		db:      db,
		cfg:     cfg,
		channel: channel,
		repo:    repository.NewInvestmentRepository(db),
	}
}

func (w *AccrualWorker) CalculateInvestmentAccrual(inv *database.Investment, days int) (int64, error) {
	parsedRate, err := strconv.ParseFloat(inv.AnnualRate, 64)
	if err != nil {
		return 0, err
	}
	// accrual is calculated monthly rounded to two decimal places
	accrualAmount := int64(float64(inv.PrincipalInitial) * parsedRate * float64(days) / 365)
	return accrualAmount, nil
}

func (w *AccrualWorker) ProcessInvestmentAccrual(ctx context.Context, inv *database.Investment) error {
	// Calculate days since last accrual
	days := int(time.Since(inv.LastAccrualAt.Time).Hours() / 24)
	if days <= 0 {
		return nil
	}
	accrualAmount, err := w.CalculateInvestmentAccrual(inv, days)
	if err != nil {
		return err
	}
	// next accrual in a month
	nextAccrualDate := time.Now().AddDate(0, 1, 0)
	// Update investment with new accrued interest and next accrual date

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	err = w.repo.UpdateInvestmentTx(ctx, tx, database.Investment{
		AccruedInterest:  inv.AccruedInterest + accrualAmount,
		LastAccrualAt:    sql.NullTime{Time: time.Now(), Valid: true},
		ID:               inv.ID,
		ClientID:         inv.ClientID,
		NextAccrualAt:    nextAccrualDate,
		UpdatedAt:        time.Now(),
		Status:           inv.Status,
		PrincipalCurrent: inv.PrincipalCurrent,
	})
	if err != nil {
		return fmt.Errorf("failed to update investment: %w", err)
	}
	cmd := commands.InvestmentAccruedPayload{
		InvestmentId:    inv.ID.String(),
		AccrualAmount:   accrualAmount,
		NewAccruedTotal: inv.AccruedInterest + accrualAmount,
		NextAccrualDate: nextAccrualDate.String(),
	}
	command, err := commands.NewCommand(commands.InvestmentAccrued, cmd)
	if err != nil {
		return err
	}
	err = w.channel.Publish("", w.cfg.Queues.AccrualNotice, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        command.Payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
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
