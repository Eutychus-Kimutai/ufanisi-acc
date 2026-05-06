package investment

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	httphandler "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/httpHandler"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
)

func main() {
	// Load configuration
	cfg, err := rabbitmq.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	DB_URL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatalf("Error opening db: %s", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to db: %s", err)
	}
	defer db.Close()

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

	worker, err := NewWorker(db, ch, cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// HTTPHandler
	handler := httphandler.NewHandler(worker)
	go func() {
		log.Println("Starting HTTP server on :8082")
		if err := http.ListenAndServe(":8082", handler); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	log.Println("Starting RabbitMQ consumer...")
	err = StartConsumer(context.Background(), ch, cfg.Queues.Investment, worker)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
}
