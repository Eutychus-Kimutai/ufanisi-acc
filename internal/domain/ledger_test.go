package domain

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostTransaction_UnbalancedReturnsError(t *testing.T) {
	ledger := NewLedgerService(nil, nil)

	err := ledger.PostTransaction(context.Background(), Transaction{
		Type: "test",
		Entries: []Entry{
			{AccountId: uuid.New(), Type: Debit, Amount: 100},
			{AccountId: uuid.New(), Type: Credit, Amount: 50},
		},
	})

	require.ErrorIs(t, err, ErrUnbalancedTransaction)
}
