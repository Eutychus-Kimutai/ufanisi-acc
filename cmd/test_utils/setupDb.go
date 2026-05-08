package testutils

import (
	"database/sql"
	"os"

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
	if err = db.Ping(); err != nil {
		return nil, err
	}

	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto;`); err != nil {
		return nil, err
	}

	if err = migrations.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil

}
