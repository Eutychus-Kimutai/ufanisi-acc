package repository

import (
	"context"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type Client struct {
	ID         string
	Name       string
	ClientType string
}

func (r *LedgerRepository) GetClientByID(ctx context.Context, clientID uuid.UUID) (database.Client, error) {
	client, err := r.db.GetClientByID(ctx, clientID)
	if err != nil {
		return database.Client{}, err
	}
	return database.Client{
		ID:         client.ID,
		Name:       client.Name,
		ClientType: client.ClientType,
	}, nil
}
