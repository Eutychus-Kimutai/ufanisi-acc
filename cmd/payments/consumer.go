package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	amqp "github.com/rabbitmq/amqp091-go"
)

func Consumer(ctx context.Context, ch *amqp.Channel, queueName string, handler *Handler) error {
	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down consumer...")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Message channel closed, shutting down consumer...")
					return
				}
				var event commands.Command
				err := json.Unmarshal(msg.Body, &event)
				if err != nil {
					log.Printf("Failed to unmarshal message: %v", err)
					msg.Nack(false, false)
					continue
				}
				var payload commands.PaymentResolvedPayload
				err = json.Unmarshal(event.Payload, &payload)
				if err != nil {
					log.Printf("Failed to unmarshal payload: %v", err)
					msg.Nack(false, false)
					continue
				}
				log.Printf("Received payment event: %+v", payload)
				_, err = handler.paymentRepo.TryCompletePayment(ctx, payload.IdempotencyKey)
				if err != nil {
					log.Printf("Failed to update payment status: %v", err)
					msg.Nack(false, true)
					continue
				}
				log.Printf("Payment updated to completed for external ID: %s", payload.IdempotencyKey)

			}

		}
	}()
	return nil
}
