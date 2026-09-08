package investment

import (
	"context"
	"fmt"
	"log"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxDispatcher struct {
	repo    *repository.OutboxRepository
	channel Publisher
	cfg     *rabbitmq.RabbitConfig
	locker  string
}

func NewOutboxDispatcher(repo *repository.OutboxRepository, channel Publisher, cfg *rabbitmq.RabbitConfig) *OutboxDispatcher {
	return &OutboxDispatcher{
		repo:    repo,
		channel: channel,
		cfg:     cfg,
		locker:  "investment_dispatcher",
	}
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) error {
	err := d.repo.ReleaseStaleLocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to release stale locks: %v", err)
	}
	messages, err := d.repo.ClaimPendingMessages(ctx, d.locker)
	if err != nil {
		return fmt.Errorf("failed to claim pending messages: %v", err)
	}
	for _, msg := range messages {
		if msg.AggregateType != "investment" {
			log.Printf("Skipping message ID %s with aggregate type %s\n", msg.ID, msg.AggregateType)
			continue
		}
		var queueName = ""
		switch msg.CommandType {
		case string(commands.InvestmentAccrued):
			queueName = d.cfg.Queues.InvestmentAccrued
		case string(commands.PaymentResolved):
			queueName = d.cfg.Queues.Resolved
		case string(commands.UnresolvedPayment):
			queueName = d.cfg.Queues.Unresolved
		default:
			log.Printf("Skipping message ID %s with unknown event type %s\n", msg.ID, msg.CommandType)
			continue
		}

		err = d.channel.Publish(
			"",
			queueName,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        []byte(msg.Payload),
			},
		)

		if err != nil {
			log.Printf("Failed to publish message ID %s: %v\n", msg.ID, err)
			markErr := d.repo.MarkMessageAsFailed(ctx, msg.ID, err.Error())
			if markErr != nil {
				log.Printf("Failed to mark message ID %s as failed: %v\n", msg.ID, markErr)
			}
			continue
		}
		log.Printf("Successfully published message ID %s\n", msg.ID)
		err = d.repo.MarkMessagesAsPublished(ctx, msg.ID)
		if err != nil {
			log.Printf("Failed to mark message ID %s as published: %v\n", msg.ID, err)
		}
	}
	return nil
}
