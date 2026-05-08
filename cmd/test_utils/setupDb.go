package testutils

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/Eutychus-Kimutai/ufanisi-acc/sql/migrations"
	"github.com/joho/godotenv"
)

func SetupTestDB() (*sql.DB, error) {
	// Load environment variables from .env file
	godotenv.Load("../../.env")
	DB_URL := os.Getenv("DB_URL")

	// Connect to the test database
	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto;`); err != nil {
		return nil, err
	}

	if err = migrations.Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil

}
