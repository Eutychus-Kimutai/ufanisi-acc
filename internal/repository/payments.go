package repository

import (
	"context"
	"database/sql"

	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/google/uuid"
)

type PaymentsRepository struct {
	db *database.Queries
}

func NewPaymentsRepository(db *sql.DB) *PaymentsRepository {
	return &PaymentsRepository{db: database.New(db)}
}

func (r *PaymentsRepository) WithTx(tx *sql.Tx) *PaymentsRepository {
	return &PaymentsRepository{
		db: r.db.WithTx(tx),
	}
}

func (r *PaymentsRepository) CreatePayment(ctx context.Context, payment database.Payment) (database.Payment, error) {
	payment, err := r.db.CreatePayment(ctx, database.CreatePaymentParams{
		ClientRef:      payment.ClientRef,
		Amount:         payment.Amount,
		PaymentType:    payment.PaymentType,
		ExternalID:     payment.ExternalID,
		IdempotencyKey: payment.IdempotencyKey,
		PaymentRef:     payment.PaymentRef,
		Destination:    payment.Destination,
		RawEvent:       payment.RawEvent,
	})
	if err != nil {
		return database.Payment{}, err
	}
	return payment, nil
}

func (r *PaymentsRepository) CreateUnresolvedPayment(ctx context.Context, payment database.UnresolvedPayment) error {
	err := r.db.CreateUnresolvedPayment(ctx, database.CreateUnresolvedPaymentParams{
		ClientRef:      payment.ClientRef,
		Amount:         payment.Amount,
		PaymentChannel: payment.PaymentChannel,
		ExternalID:     payment.ExternalID,
		RawEvent:       payment.RawEvent,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *PaymentsRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (database.Payment, error) {
	payment, err := r.db.GetPaymentByID(ctx, id)
	if err != nil {
		return database.Payment{}, err
	}
	return payment, nil
}

func (r *PaymentsRepository) UpdatePaymentStatus(ctx context.Context, status string, id uuid.UUID) error {
	err := r.db.UpdatePaymentStatus(ctx, database.UpdatePaymentStatusParams{
		Status: status,
		ID:     id,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *PaymentsRepository) ResolvePayment(ctx context.Context, id uuid.UUID, status string, resolvedType sql.NullString) error {
	err := r.db.ResolvePayment(ctx, database.ResolvePaymentParams{
		ID:           id,
		Status:       status,
		ResolvedType: resolvedType,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *PaymentsRepository) GetUnresolvedPayments(ctx context.Context, limit int32) ([]database.Payment, error) {
	payments, err := r.db.GetUnresolvedPayments(ctx, limit)
	if err != nil {
		return nil, err
	}
	return payments, nil
}

func (r *PaymentsRepository) ListPayments(ctx context.Context, limit int32, offset int32) ([]database.Payment, error) {
	payments, err := r.db.ListPayments(ctx, database.ListPaymentsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	return payments, nil
}

func (r *PaymentsRepository) GetPaymentByIdempotencyKey(ctx context.Context, key string) (database.Payment, error) {
	payment, err := r.db.GetPaymentByIdempotencyKey(ctx, key)
	if err != nil {
		return database.Payment{}, err
	}
	return payment, nil
}
func (r *PaymentsRepository) CreatePaymentReference(ctx context.Context, reference database.PaymentReference) error {
	_, err := r.db.CreatePaymentReference(ctx, database.CreatePaymentReferenceParams{
		Reference:  reference.Reference,
		EntityType: reference.EntityType,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *PaymentsRepository) GetPaymentReference(ctx context.Context, reference string) (database.PaymentReference, error) {

	paymentRef, err := r.db.GetPaymentReference(ctx, reference)
	if err != nil {
		return database.PaymentReference{}, err
	}
	return paymentRef, nil
}

func (r *PaymentsRepository) TryClaimPayment(ctx context.Context, idempotencyKey string) (database.Payment, error) {
	payments, err := r.db.TryClaimPayment(ctx, idempotencyKey)
	if err != nil {
		return database.Payment{}, err
	}
	return payments, nil
}

func (r *PaymentsRepository) TryCompletePayment(ctx context.Context, idempotencyKey string) (database.Payment, error) {
	payments, err := r.db.TryCompletePayment(ctx, idempotencyKey)
	if err != nil {
		return database.Payment{}, err
	}
	return payments, nil
}

func (r *PaymentsRepository) TryFailPayment(ctx context.Context, idempotencyKey string) (database.Payment, error) {
	payments, err := r.db.TryFailPayment(ctx, idempotencyKey)
	if err != nil {
		return database.Payment{}, err
	}
	return payments, nil
}
func (r *PaymentsRepository) TryUnresolvePayment(ctx context.Context, idempotencyKey string) (database.Payment, error) {
	payments, err := r.db.TryUnresolvePayment(ctx, idempotencyKey)
	if err != nil {
		return database.Payment{}, err
	}
	return payments, nil
}
