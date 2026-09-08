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

func NewClientRepository(db *database.Queries) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) GetClientByID(ctx context.Context, clientID uuid.UUID) (database.Client, error) {

	client, err := r.db.GetClientByID(ctx, clientID)
	if err != nil {
		return database.Client{}, err
	}
	return client, nil
}
