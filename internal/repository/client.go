package repository

import (
	"context"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type ClientRepository struct {
	db *database.Queries
}
type Client struct {
	ID         string
	Name       string
	ClientType string
}

// NewClientRepository creates a client repository backed by db.
func NewClientRepository(db *database.Queries) *ClientRepository {
	return &ClientRepository{db: db}
}

// GetClientByID returns the client identified by clientID.
func (r *ClientRepository) GetClientByID(ctx context.Context, clientID uuid.UUID) (database.Client, error) {

	client, err := r.db.GetClientByID(ctx, clientID)
	if err != nil {
		return database.Client{}, err
	}
	return client, nil
}
