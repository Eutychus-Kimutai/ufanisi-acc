package rabbitmq

import (
	"fmt"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

func NewConnection(cfg *RabbitConfig) (*amqp.Connection, error) {
	connStrring := fmt.Sprintf("amqp://%s:%s@%s:%d/%s", cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Vhost)
	conn, err := amqp.Dial(connStrring)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %s\n", err)
		return nil, fmt.Errorf("Connection to rabbitmq failed: %s", err)
	}

	return conn, nil

}
func NewChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %s\n", err)
		return nil, fmt.Errorf("Failed to open a channel: %s", err)
	}

	return ch, nil
}

func LoadConfig(path string) (*RabbitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read config file: %s\n", err)
		return nil, err
	}
	var cfg RabbitConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		log.Printf("Failed to unmarshal config: %s\n", err)
		return nil, err
	}
	return &cfg, nil
}

func QueueDeclare(ch *amqp.Channel, cfg *RabbitConfig) error {
	_, err := ch.QueueDeclare(
		cfg.Queues.Loan,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": cfg.Queues.Loan + ".dlq",
		},
	)
	if err != nil {
		log.Printf("Failed to declare loan queue: %s\n", err)
		return fmt.Errorf("Failed to declare loan queue: %s", err)
	}
	_, err = ch.QueueDeclare(
		cfg.Queues.Loan+".dlq",
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		log.Printf("Failed to declare loan DLQ: %s\n", err)
		return fmt.Errorf("Failed to declare loan DLQ: %s", err)
	}
	_, err = ch.QueueDeclare(
		cfg.Queues.Investment,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": cfg.Queues.Investment + ".dlq",
		},
	)
	if err != nil {
		log.Printf("Failed to declare investment queue: %s\n", err)
		return fmt.Errorf("Failed to declare investment queue: %s", err)
	}
	_, err = ch.QueueDeclare(
		cfg.Queues.Investment+".dlq",
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		log.Printf("Failed to declare investment DLQ: %s\n", err)
		return fmt.Errorf("Failed to declare investment DLQ: %s", err)
	}
	return nil
}
