package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/rabbitmq"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/transport"
	"github.com/Eutychus-Kimutai/ufanisi-acc/sql/migrations"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	DB_URL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	err = migrations.Migrate(db)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	repo := repository.NewRepository(db)
	ledgerService := domain.NewLedgerService(db, repo)
	investmentRepo := repository.NewInvestmentRepository(db)

	// Initialize RabbitMQ publisher
	cfg, err := rabbitmq.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load RabbitMQ config: %v", err)
	}
	conn, err := rabbitmq.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := rabbitmq.NewChannel(conn)
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v", err)
	} 
	defer ch.Close()

	router := transport.NewRouter(db, ledgerService, investmentRepo, ch)
	http.ListenAndServe(":8080", router)
}
