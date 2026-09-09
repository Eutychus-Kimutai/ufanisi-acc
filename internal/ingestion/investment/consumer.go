package investment

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// StartConsumer consumes investment payment events until the context is canceled.
func StartConsumer(ctx context.Context, ch *amqp.Channel, queueName string, worker *Worker) error {
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
				var payload commands.ResolvePaymentPayload
				err = json.Unmarshal(event.Payload, &payload)
				if err != nil {
					log.Printf("Failed to unmarshal payload: %v", err)
					msg.Nack(false, false)
					continue
				}
				log.Printf("Received payment event: %+v", payload)
				_, err = worker.paymentRepo.TryClaimPayment(ctx, payload.ExternalId)
				if err != nil {
					log.Printf("Failed to get payment processing status: %v", err)
					if err == sql.ErrNoRows {
						log.Printf("Payment with external ID %s was not claimed; already processing or not eligible", payload.ExternalId)
						msg.Ack(false) // Acknowledge the message to prevent infinite requeueing
						continue
					}
					msg.Nack(false, true)
					continue
				}

				err = worker.HandlePaymentEvent(ctx, payload)
				if err != nil {
					log.Printf("Failed to handle payment event: %v", err)
					if _, failErr := worker.paymentRepo.TryFailPayment(ctx, payload.ExternalId); failErr != nil {
						log.Printf("Failed to mark payment as failed: %v", failErr)
					}

					msg.Nack(false, false)

					continue
				}

				msg.Ack(false)
			}
		}
	}()
	return nil
}
