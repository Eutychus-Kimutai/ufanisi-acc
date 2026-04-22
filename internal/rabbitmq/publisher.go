package rabbitmq

import (
	"encoding/json"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishCommand(ch *amqp.Channel, queueName string, cmd commands.Command) error {
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return ch.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
