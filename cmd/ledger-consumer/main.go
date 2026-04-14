package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/commands"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg, err := rabbitmq.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	conn, err := rabbitmq.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	err = rabbitmq.QueueDeclare(ch, cfg)
	if err != nil {
		log.Fatalf("Failed to declare queues: %v", err)
	}

	DB_URL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	repo := repository.NewRepository(db)
	ledgerService := domain.NewLedgerService(db, repo)

	consumer := NewConsumer(ch, ledgerService, cfg.Retry.MaxRetries, time.Duration(cfg.Retry.DelaySeconds)*time.Second)

	log.Println("Starting consumer...")
	err = consumer.Start(cfg.Queues.Loan)
	if err != nil {
		log.Fatalf("Consumer error: %v", err)
	}
}

type Consumer struct {
	channel    *amqp.Channel
	ledger     *domain.LedgerService
	maxRetries int
	delay      time.Duration
}

func NewConsumer(ch *amqp.Channel, ledger *domain.LedgerService, maxRetries int, delay time.Duration) *Consumer {
	return &Consumer{
		channel:    ch,
		ledger:     ledger,
		maxRetries: maxRetries,
		delay:      delay,
	}
}

func (c *Consumer) Start(queueName string) error {
	msgs, err := c.channel.Consume(
		queueName,
		"",
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %s\n", err)
		return err
	}
	for msg := range msgs {
		err := c.processMessage(msg)
		if err != nil {
			log.Printf("Failed to process message: %s\n", err)
			err = c.handleRetry(queueName, msg, err)
			if err != nil {
				log.Printf("Failed to handle retry: %s\n", err)
			}
		} else {
			msg.Ack(false)
		}
	}
	return nil
}

func (c *Consumer) processMessage(msg amqp.Delivery) error {
	var cmd commands.Command
	err := json.Unmarshal(msg.Body, &cmd)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal message: %s", err)
	}
	entries := make([]domain.Entry, len(cmd.Payload.Entries))
	for i, e := range cmd.Payload.Entries {
		parsedId, err := uuid.Parse(e.AccountID)
		if err != nil {
			return fmt.Errorf("Failed to parse account ID: %s", err)
		}

		entries[i] = domain.Entry{
			AccountId: parsedId,
			Amount:    e.Amount,
			Type:      domain.EntryType(e.Type),
		}
	}
	switch cmd.Type {
	case commands.PostTransaction:
		return c.ledger.PostTransaction(context.Background(), domain.Transaction{
			Reference: cmd.Payload.Reference,
			Entries:   entries})
	default:
		return fmt.Errorf("Unknown command type: %s", cmd.Type)
	}
}

func (c *Consumer) handleRetry(queueName string, msg amqp.Delivery, processErr error) error {
	operation := func() error {
		return c.processMessage(msg)
	}
	b := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(c.delay),
		backoff.WithMaxInterval(c.delay*10),
		backoff.WithMaxElapsedTime(c.delay*time.Duration(c.maxRetries)),
	)
	err := backoff.Retry(operation, b)
	if err == nil {
		return msg.Ack(false)
	}
	// move failed message to DLQ
	dlqEvent := map[string]interface{}{
		"payload":   string(msg.Body),
		"failed_at": time.Now(),
		"error":     processErr.Error(),
	}
	dlqBody, err := json.Marshal(dlqEvent)
	if err != nil {
		log.Printf("Failed to marshal DLQ event: %s\n", err)
		return fmt.Errorf("Failed to marshal DLQ event: %s", err)
	}
	err = c.channel.Publish(
		"",               // exchange
		queueName+".dlq", // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        dlqBody,
		},
	)
	if err != nil {
		log.Printf("Failed to publish to DLQ: %s\n", err)
		return fmt.Errorf("Failed to publish to DLQ: %s", err)
	}
	return msg.Nack(false, false) // dont requeue, message is now in DLQ
}
