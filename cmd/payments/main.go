package main

import (
	"context"
	"database/sql"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening db: %s", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to db from payments: %s", err)
	}
	defer db.Close()

	// Load Rabbitmq configuration
	cfg, err := rabbitmq.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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

	handler, err := NewHandler(db, ch, cfg)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}
	err = Consumer(context.Background(), ch, cfg.Queues.Resolved, handler)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
	err = Consumer(context.Background(), ch, cfg.Queues.Unresolved, handler)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// Start HTTP server
		log.Println("Starting HTTP server on :8083")
		err := http.ListenAndServe(":8083", PaymentsRouter(handler))
		if err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}

	}()
	<-sigChan
}
