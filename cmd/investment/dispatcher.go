package main

import (
	"context"
	"fmt"
	"log"

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
	log.Println("Starting outbox dispatch cycle")
	messages, err := d.repo.ClaimPendingMessages(ctx, d.locker)
	if err != nil {
		return fmt.Errorf("failed to claim pending messages: %v", err)
	}
	for _, msg := range messages {
		err := d.channel.Publish(
			"",
			d.cfg.Queues.AccrualNotice,
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
