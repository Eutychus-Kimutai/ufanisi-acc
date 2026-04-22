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

func (r *LedgerRepository) GetClientByID(ctx context.Context, clientID string) (database.Client, error) {
	parsedClientID, err := uuid.Parse(clientID)
	if err != nil {
		return database.Client{}, err
	}
	client, err := r.db.GetClientByID(ctx, parsedClientID)
	if err != nil {
		return database.Client{}, err
	}
	return database.Client{
		ID:         client.ID,
		Name:       client.Name,
		ClientType: client.ClientType,
	}, nil
}
