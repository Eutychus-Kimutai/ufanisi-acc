package loanworker

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
)

func main() {
	// Load configuration
	cfg, err := rabbitmq.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	DB_URL := os.Getenv("DB_URL")
	db, err := sql.Open("Postgres", DB_URL)
	if err != nil {
		log.Fatalf("Error opening db: %s", err)
	}

	// Connect to RabbitMQ
	conn, err := rabbitmq.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Open a channel
	ch, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare the queue
	err = rabbitmq.QueueDeclare(ch, cfg)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// repo := repository.NewRepository(db)

	worker, err := NewWorker(db, ch, "payments.loan", cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	HTTPHandler := NewHTTPHandler(worker)

	go func() {
		log.Println("Starting HTTP server on :8080")
		http.ListenAndServe(":8080", HTTPHandler)
	}()

	log.Println("Starting RabbitMQ consumer...")
	err = StartConsumer(ch, "payments.loan", worker)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
}
