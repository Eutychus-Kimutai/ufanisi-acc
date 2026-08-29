package loan

import (
	"context"
	"encoding/json"
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
		locker:  "loan_dispatcher",
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
		if msg.AggregateType != "loan" {
			log.Printf("Skipping message ID %s with aggregate type %s\n", msg.ID, msg.AggregateType)
			continue
		}
		var cmd commands.Command
		err := json.Unmarshal([]byte(msg.Payload), &cmd)
		if err != nil {
			log.Printf("Failed to unmarshal message ID %s: %v\n", msg.ID, err)
			markErr := d.repo.MarkMessageAsFailed(ctx, msg.ID, err.Error())
			if markErr != nil {
				log.Printf("Failed to mark message ID %s as failed: %v\n", msg.ID, markErr)
			}
			continue
		}

		queName := ""
		switch cmd.Type {
		case commands.PaymentResolved:
			queName = d.cfg.Queues.Resolved
			log.Printf("Dispatching message ID %s to queue %s\n", msg.ID, queName)
		case commands.UnresolvedPayment:
			queName = d.cfg.Queues.Unresolved
			log.Printf("Dispatching message ID %s to queue %s\n", msg.ID, queName)
		default:
			log.Printf("Skipping message ID %s with unknown event type %s\n", msg.ID, msg.AggregateType)
			continue
		}

		err = d.channel.Publish(
			"",
			queName,
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
		log.Printf("Successfully published message ID %s to queue %s\n", msg.ID, queName)
		err = d.repo.MarkMessagesAsPublished(ctx, msg.ID)
		if err != nil {
			log.Printf("Failed to mark message ID %s as published: %v\n", msg.ID, err)
		}
	}
	return nil
}
