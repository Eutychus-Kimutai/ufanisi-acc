package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httphandler "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/httpHandler"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
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
	if err = db.PingContext(context.Background()); err != nil {
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dispacherCh, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open channel for dispatcher: %v", err)
	}
	defer dispacherCh.Close()
	dispatcher := &OutboxDispatcher{
		repo:    repository.NewOutboxRepository(db),
		channel: dispacherCh,
		locker:  "investment-dispatcher",
		cfg:     cfg,
	}
	log.Println("Starting outbox dispatcher...")
	log.Println("Purging failed messages...")
	purgeOnce := func(runCtx context.Context) {
		deletedCount, err := dispatcher.repo.PurgeOldMessages(runCtx, 10, 100)
		if err != nil {
			log.Printf("Error purging old messages: %v", err)
		} else {
			log.Printf("Purged %d old failed messages", deletedCount)
		}
	}
	go func() {
		purgeOnce(ctx)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Get fresh context to avoid using a canceled context
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
				log.Println("Dispatcher stopping...")
				return
			case <-ticker.C:
				err := dispatcher.DispatchOnce(ctx)
				if err != nil {
					log.Printf("Error dispatching messages: %v", err)
				}
			}
		}
	}()
	go func() {
		log.Println("Starting HTTP server on :8082")
		if err := http.ListenAndServe(":8082", handler); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	log.Println("Starting RabbitMQ consumer...")
	err = StartConsumer(ctx, ch, cfg.Queues.Investment, worker)
	if err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
	// New channel for scheduler
	schedulerCh, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open channel for scheduler: %v", err)
	}

	log.Println("Starting interest accrual scheduler...")
	accrualWorker := NewAccrualWorker(db, schedulerCh, cfg)
	err = StartScheduler(ctx, worker, accrualWorker)
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer schedulerCh.Close()
	<-ctx.Done()

}
