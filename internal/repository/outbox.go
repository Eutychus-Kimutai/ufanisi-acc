package repository

import (
	"context"
	"database/sql"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type OutboxRepository struct {
	db *database.Queries
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: database.New(db)}
}

func (r *OutboxRepository) WithTx(tx *sql.Tx) *OutboxRepository {
	return &OutboxRepository{
		db: r.db.WithTx(tx),
	}
}

func (r *OutboxRepository) CreateOutboxMessage(ctx context.Context, msg database.OutboxMessage) error {
	_, err := r.db.CreateOutboxMessage(ctx, database.CreateOutboxMessageParams{
		AggregateType: msg.AggregateType,
		AggregateID:   msg.AggregateID,
		Payload:       msg.Payload,
		CommandType:   msg.CommandType,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *OutboxRepository) ClaimPendingMessages(ctx context.Context, lockedBy string) ([]database.OutboxMessage, error) {
	messages, err := r.db.ClaimPendingMessages(ctx, database.ClaimPendingMessagesParams{
		LockedBy: sql.NullString{String: lockedBy, Valid: true},
		Limit:    10,
	})
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *OutboxRepository) MarkMessagesAsPublished(ctx context.Context, messageIDs uuid.UUID) error {
	err := r.db.MarkMessageAsPublished(ctx, messageIDs)
	if err != nil {
		return err
	}
	return nil
}

func (r *OutboxRepository) MarkMessageAsFailed(ctx context.Context, messageID uuid.UUID, errMsg string) error {
	err := r.db.MarkMessageAsFailed(ctx, database.MarkMessageAsFailedParams{
		ID:        messageID,
		LastError: sql.NullString{String: errMsg, Valid: true},
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *OutboxRepository) ReleaseStaleLocks(ctx context.Context) error {
	err := r.db.ReleaseStaleLocks(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *OutboxRepository) PurgeOldMessages(ctx context.Context, days, limit int) (int64, error) {
	deletedMessages, err := r.db.PurgeOldMessages(ctx, database.PurgeOldMessagesParams{
		Days:  int32(days),
		Limit: int32(limit),
	})
	if err != nil {
		return 0, err
	}
	return deletedMessages, nil
}
