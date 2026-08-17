package main

import (
	"context"
	"database/sql"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"

	httphandler "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/httpHandler"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/ingestion/loan"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
)

func main() {
	godotenv.Load()
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

	// repo := repository.NewRepository(db)

	worker, err := loan.NewWorker(db, ch, cfg.Queues.Loan, cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	HTTPHandler := httphandler.NewHandler(worker)
	notifyCtx := make(chan os.Signal, 1)
	signal.Notify(notifyCtx, os.Interrupt)

	go func() {
		log.Println("Starting HTTP server on :8081")
		if err := http.ListenAndServe(":8081", HTTPHandler); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}

	}()

	log.Println("Starting RabbitMQ consumer...")
	err = loan.StartConsumer(context.Background(), ch, cfg.Queues.Loan, worker)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
	<-notifyCtx
}
