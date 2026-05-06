package testutils

import (
	"database/sql"
	"os"

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
	return db, nil
}
