package rabbitmq

import (
	"encoding/json"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

func PublishCommand(publisher Publisher, queueName string, cmd commands.Command) error {
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return publisher.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
