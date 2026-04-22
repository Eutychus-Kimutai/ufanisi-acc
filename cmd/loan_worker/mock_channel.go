package loanworker

import (
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MockChannel struct {
	PublishedMessages []PublishedMessage
}

type PublishedMessage struct {
	Queue   string
	Payload []byte
}

func (m *MockChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	body, err := json.Marshal(json.RawMessage(msg.Body))
	if err != nil {
		return err
	}
	m.PublishedMessages = append(m.PublishedMessages, PublishedMessage{
		Queue:   key,
		Payload: body,
	})
	return nil
}

func (m *MockChannel) Close() error {
	return nil
}

func (m *MockChannel) NotifyPublish(c chan amqp.Confirmation) <-chan amqp.Confirmation {
	return c
}
