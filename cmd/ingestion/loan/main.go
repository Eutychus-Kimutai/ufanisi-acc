package main

import (
	"context"
	"database/sql"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httphandler "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/httpHandler"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/ingestion/loan"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dispatcherChannel, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open dispatcher channel: %v", err)
	}
	defer dispatcherChannel.Close()
	outboxRepo := repository.NewOutboxRepository(db)

	dispatcher := loan.NewOutboxDispatcher(outboxRepo, dispatcherChannel, cfg)
	purgeOnce := func(runCtx context.Context) {
		deletedCount, purgeErr := outboxRepo.PurgeOldMessages(runCtx, 10, 100)
		if purgeErr != nil {
			log.Printf("Failed to purge old messages: %v", purgeErr)
		} else {
			log.Printf("Purged %d old messages from the outbox", deletedCount)
		}
	}

	go func() {
		purgeOnce(ctx)
		log.Println("Starting outbox dispatcher...")
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				freshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				purgeOnce(freshCtx)

				cancel()
				return

			case <-ticker.C:
				purgeOnce(ctx)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down outbox dispatcher...")
				return
			case <-ticker.C:
				err := dispatcher.DispatchOnce(ctx)
				if err != nil {
					log.Printf("Error dispatching messages: %v", err)
				}
			}
		}
	}()

	HTTPHandler := httphandler.NewHandler(worker)

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
	<-ctx.Done()
}
