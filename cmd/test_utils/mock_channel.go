package testutils

import (
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
	m.PublishedMessages = append(m.PublishedMessages, PublishedMessage{
		Queue:   key,
		Payload: msg.Body,
	})

	return nil
}

func (m *MockChannel) Close() error {
	return nil
}

func (m *MockChannel) NotifyPublish(c chan amqp.Confirmation) <-chan amqp.Confirmation {
	return c
}
