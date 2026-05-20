package main

import (
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	amqp "github.com/rabbitmq/amqp091-go"
)

func (a *AccrualWorker) GenerateAccrualNotice(inv *database.Investment, accruedAmount int64) error {
	noticePayload := commands.AccrualNoticePayload{
		InvestmentId:  inv.ID.String(),
		AccrualAmount: accruedAmount,
	}
	command, err := commands.NewCommand(commands.InvestmentAccrued, noticePayload)
	if err != nil {
		return err
	}
	err = a.channel.Publish("", a.cfg.Queues.AccrualNotice, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        command.Payload,
	})
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) GenerateWithdrawalNotice(inv *database.Investment, amount int64) error {
	noticePayload := commands.InvestmentWithdrawalRequestedPayload{
		InvestmentId: inv.ID.String(),
		Amount:       amount,
	}
	command, err := commands.NewCommand(commands.InvestmentWithdrawalRequested, noticePayload)
	if err != nil {
		return err
	}
	err = w.channel.Publish("", w.cfg.Queues.WithdrawalNotice, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        command.Payload,
	})
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) GenerateMaturityNotice(inv *database.Investment) error {
	noticePayload := commands.InvestmentMaturedPayload{
		InvestmentId: inv.ID.String(),
	}
	command, err := commands.NewCommand(commands.InvestmentMatured, noticePayload)
	if err != nil {
		return err
	}
	err = w.channel.Publish("", w.cfg.Queues.MaturityNotice, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        command.Payload,
	})
	if err != nil {
		return err
	}
	return nil
}
