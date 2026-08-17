package loan

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func StartConsumer(ctx context.Context, ch *amqp.Channel, queueName string, worker *LoanWorker) error {
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

				_, err = worker.paymentRepo.TryClaimPayment(ctx, payload.ExternalId)
				if err != nil {
					log.Printf("Failed to get payment processing status: %v", err)
					if err == sql.ErrNoRows {
						log.Printf("Payment with external ID %s was not claimed; already processing or not eligible", payload.ExternalId)
						msg.Ack(false)
						continue
					}
					msg.Nack(false, true)
					continue
				}

				err = worker.HandlePaymentEvent(ctx, payload)
				if err != nil {

					if err != ErrLoanNotFound {
						if _, failErr := worker.paymentRepo.TryFailPayment(ctx, payload.ExternalId); failErr != nil {
							log.Printf("Failed to update payment status to completed: %v", failErr)
						}
					}

					if _, unresolvedErr := worker.paymentRepo.TryUnresolvePayment(ctx, payload.ExternalId); unresolvedErr != nil {
						log.Printf("Failed to update payment status to unresolved: %v", unresolvedErr)
					}
					cmd := commands.UnresolvedPaymentPayload{
						Amount:     payload.Amount,
						ClientRef:  payload.ClientRef,
						ExternalId: payload.ExternalId,
						Reason:     err.Error(),
					}
					unresolvedCmd, err := commands.NewCommand(
						commands.UnresolvedPayment,
						cmd,
					)
					if err != nil {
						log.Printf("failed to create unresolved payment command: %v", err)
					}
					err = rabbitmq.PublishCommand(
						worker.channel,
						worker.cfg.Queues.Unresolved,
						unresolvedCmd,
					)
					if err != nil {
						log.Printf("failed to publish unresolved payment command: %v", err)
					}
					log.Printf("Published unresolved payment command for ExternalId: %s", payload.ExternalId)

					msg.Nack(false, false)
					continue
				}

				msg.Ack(false)
			}
		}
	}()
	return nil
}
