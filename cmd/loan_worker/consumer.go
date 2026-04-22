package loanworker

import (
	"context"
	"encoding/json"
	"log"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/payment"
	amqp "github.com/rabbitmq/amqp091-go"
)

func StartConsumer(ch *amqp.Channel, queueName string, worker *Worker) error {
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
		for msg := range msgs {
			var event payment.PaymentEvent
			err := json.Unmarshal(msg.Body, &event)
			if err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				msg.Nack(false, false)
				continue
			}

			err = worker.HandlePaymentEvent(context.Background(), event)
			if err != nil {
				log.Printf("Failed to handle payment event: %v", err)
				msg.Nack(false, false)
				continue
			}

			msg.Ack(false)
			continue
		}
	}()

	return nil
}
