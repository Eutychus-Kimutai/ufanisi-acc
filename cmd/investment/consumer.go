package investment

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	amqp "github.com/rabbitmq/amqp091-go"
)

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
				var event payment.PaymentEvent
				err := json.Unmarshal(msg.Body, &event)
				if err != nil {
					log.Printf("Failed to unmarshal message: %v", err)
					msg.Nack(false, false)
					continue
				}

				err = worker.HandlePaymentEvent(ctx, event)
				if err != nil {
					log.Printf("Failed to handle payment event: %v", err)
					msg.Nack(false, false)

					continue
				}

				msg.Ack(false)
			}
		}
	}()
	return nil
}
