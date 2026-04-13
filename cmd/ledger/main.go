package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/domain"
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
	router := transport.NewRouter(ledgerService)
	http.ListenAndServe(":8080", router)
}
