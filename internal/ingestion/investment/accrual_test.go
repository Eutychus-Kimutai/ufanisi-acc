package investment

import (
	"context"
	"database/sql"
	"testing"
	"time"

	testutils "github.com/Eutychus-Kimutai/ufanisi-acc/cmd/test_utils"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/database"
	"github.com/Eutychus-Kimutai/ufanisi-acc/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccrual(t *testing.T) {
	setup := func(t *testing.T) (*AccrualWorker, *database.Investment, *OutboxDispatcher, *testutils.MockChannel, func()) {
		db, dbCleanup := SetupDBWithCleanup(t)
		require.NotNil(t, db)

		mockCh := &testutils.MockChannel{}

		_, dispatcher, accrualWorker := BuildWorkerAndDispatcher(t, db, mockCh, database.New(db))

		accrualClientID := uuid.New()
		t.Logf("Creating accrual client with ID: %s", accrualClientID)

		// Create accrual client
		_, err := db.ExecContext(context.Background(),
			`INSERT INTO clients (id, name, client_type) VALUES ($1, $2, $3)`,
			accrualClientID, "Accrual Client", "investment")
		require.NoError(t, err)

		investment := &database.Investment{
			ID:               uuid.New(),
			ClientID:         accrualClientID,
			PrincipalInitial: 100000,
			PrincipalCurrent: 100000,
			MonthlyRate:      "2.5",
			Status:           "active",
			Reference:        "INVEST-ACCRUAL-001",
			AccruedInterest:  0,
			LastAccrualAt:    sql.NullTime{Time: time.Now().AddDate(0, -1, 0), Valid: true},
			NextAccrualAt:    time.Now().AddDate(0, 0, 1),
		}

		_, err = db.ExecContext(
			context.Background(),
			`INSERT INTO investments (
            id, client_id, principal_initial, principal_current, monthly_rate, status, reference,
            accrued_interest, last_accrual_at, next_accrual_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, $10)`,
			investment.ID,
			investment.ClientID,
			investment.PrincipalInitial,
			investment.PrincipalCurrent,
			investment.MonthlyRate,
			investment.Status,
			investment.Reference,
			investment.AccruedInterest,
			investment.LastAccrualAt,
			investment.NextAccrualAt,
		)
		require.NoError(t, err)
		cleanup := func() {
			defer dbCleanup()
			ctx := context.Background()
			_, err = db.ExecContext(ctx, "DELETE FROM clients WHERE id = $1", investment.ClientID)
			require.NoError(t, err)

			_, err := db.ExecContext(ctx, "DELETE FROM investments WHERE id = $1", investment.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `DELETE FROM outbox_messages WHERE aggregate_id = $1`, investment.ID)
			require.NoError(t, err)
		}
		return accrualWorker, investment, dispatcher, mockCh, cleanup
	}
	t.Run("Test with multiple months", func(t *testing.T) {
		t.Parallel()
		accrualWorker, investment, _, _, cleanup := setup(t)
		// get investment account and client IDs
		inv, err := accrualWorker.repo.GetInvestmentByReference(context.Background(), investment.Reference)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		t.Cleanup(cleanup)
		err = accrualWorker.ProcessInvestmentAccrual(context.Background(), investment)
		require.NoError(t, err, "Expected to process investment accrual without error")

		// Verify the investment accrual has correct values
		invRepo := repository.NewInvestmentRepository(accrualWorker.db)
		inv, err = invRepo.GetInvestmentByID(context.Background(), investment.ID)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		expectedAccruedInterest := int64(2500)
		require.Equal(t, expectedAccruedInterest, inv.AccruedInterest, "Expected AccruedInterest in database to be 2500 after accrual processing")
		expectedNextAccrualAt := time.Now().AddDate(0, 1, 0)
		assert.WithinDuration(t, expectedNextAccrualAt, inv.NextAccrualAt, 2*time.Second, "Expected NextAccrualAt in database to be one month from now")
		expectedLastAccrualAt := investment.LastAccrualAt.Time.AddDate(0, 1, 0)
		assert.WithinDuration(t, expectedLastAccrualAt, inv.LastAccrualAt.Time, 2*time.Second, "Expected LastAccrualAt in database to be one month from previous accrual")
	})

	t.Run("Test with some interest already accrued", func(t *testing.T) {
		t.Parallel()
		accrualWorker, investment, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)
		db := accrualWorker.db
		investment.AccruedInterest = 5000
		_, err := db.ExecContext(context.Background(),
			`UPDATE investments SET accrued_interest = $1 WHERE id = $2`,
			investment.AccruedInterest, investment.ID)
		require.NoError(t, err, "Expected to update accrued interest in database without error")

		err = accrualWorker.ProcessInvestmentAccrual(context.Background(), investment)
		require.NoError(t, err, "Expected to process investment accrual without error")

		// Verify the investment accrual has correct values
		invRepo := repository.NewInvestmentRepository(accrualWorker.db)
		inv, err := invRepo.GetInvestmentByID(context.Background(), investment.ID)
		require.NoError(t, err, "Expected to retrieve investment from database without error")
		expectedAccruedInterest := int64(7500)
		require.Equal(t, expectedAccruedInterest, inv.AccruedInterest, "Expected AccruedInterest in database to be 7500 after accrual processing")
		expectedNextAccrualAt := time.Now().AddDate(0, 1, 0)
		assert.WithinDuration(t, expectedNextAccrualAt, inv.NextAccrualAt, 2*time.Second, "Expected NextAccrualAt in database to be one month from now")
		expectedLastAccrualAt := investment.LastAccrualAt.Time.AddDate(0, 1, 0)
		assert.WithinDuration(t, expectedLastAccrualAt, inv.LastAccrualAt.Time, 2*time.Second, "Expected LastAccrualAt in database to be one month from previous accrual")
	})

	t.Run("Test outbox message creation", func(t *testing.T) {
		accrualWorker, investment, dispatcher, mockCh, cleanup := setup(t)
		t.Cleanup(cleanup)
		err := accrualWorker.ProcessInvestmentAccrual(context.Background(), investment)
		require.NoError(t, err, "Expected to process investment accrual without error")
		// Verify the outbox message has been created
		var (
			outboxCount         int
			outboxStatus        string
			outboxCommandType   string
			outboxAggregateID   string
			outboxAggregateType string
			outboxPayload       []byte
		)
		err = accrualWorker.db.QueryRowContext(context.Background(),
			`SELECT COUNT(*), status, command_type, aggregate_id, aggregate_type, payload
			FROM outbox_messages
			WHERE aggregate_id = $1
			GROUP BY status, command_type, aggregate_id, aggregate_type, payload`,
			investment.ID,
		).Scan(&outboxCount, &outboxStatus, &outboxCommandType, &outboxAggregateID, &outboxAggregateType, &outboxPayload)

		require.NoError(t, err, "Expected to retrieve outbox message from database without error")

		require.Equal(t, 1, outboxCount, "Expected one outbox message for the investment accrual")
		require.Equal(t, "pending", outboxStatus, "Expected outbox message status to be 'pending'")
		require.Equal(t, "INVESTMENT_ACCRUED", outboxCommandType, "Expected outbox message command type to be 'InvestmentAccrualProcessed'")
		require.Equal(t, investment.ID.String(), outboxAggregateID, "Expected outbox message aggregate ID to match investment ID")

		// Process the outbox message and verify it is marked as processed
		err = dispatcher.DispatchOnce(context.Background())
		require.NoError(t, err, "Expected DispatchOnce to complete without error")
		require.Equal(t, 1, len(mockCh.PublishedMessages), "Expected exactly one message to be published")
	})

	t.Run("Purge old outbox messages", func(t *testing.T) {
		accrualWorker, _, _, _, cleanup := setup(t)
		t.Cleanup(cleanup)

		fixtureIDs := []uuid.UUID{
			uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		}

		t.Cleanup(func() {
			for _, id := range fixtureIDs {
				_, err := accrualWorker.db.ExecContext(context.Background(), `DELETE FROM outbox_messages WHERE aggregate_id = $1`, id)
				require.NoError(t, err)
			}
		})

		_, err := accrualWorker.db.ExecContext(context.Background(), `DELETE FROM outbox_messages
		WHERE aggregate_id IN (
			'00000000-0000-0000-0000-000000000001',
			'00000000-0000-0000-0000-000000000002',
			'00000000-0000-0000-0000-000000000003',
				'00000000-0000-0000-0000-000000000004'
			)`)
		require.NoError(t, err)

		_, err = accrualWorker.db.ExecContext(context.Background(), `INSERT INTO outbox_messages (
	 aggregate_type, aggregate_id, command_type, payload, status, attempts, updated_at, created_at
	) VALUES ('investment','00000000-0000-0000-0000-000000000001','INVESTMENT_ACCRUED','{}','failed',5,NOW()-INTERVAL '20 days',NOW()-INTERVAL '20 days'),
	('investment','00000000-0000-0000-0000-000000000002','INVESTMENT_ACCRUED','{}','failed',5,NOW()-INTERVAL '15 days',NOW()-INTERVAL '15 days');`,
		)
		require.NoError(t, err, "Expected to insert old failed outbox messages without error")

		_, err = accrualWorker.db.ExecContext(context.Background(), `INSERT INTO outbox_messages (
	 aggregate_type, aggregate_id, command_type, payload, status, attempts, updated_at, created_at
	 ) VALUES
	 ('investment','00000000-0000-0000-0000-000000000003','INVESTMENT_ACCRUED','{}','failed',4,NOW()-INTERVAL '20 days',NOW()-INTERVAL '20 days'),
	 ('investment','00000000-0000-0000-0000-000000000004','INVESTMENT_ACCRUED','{}','failed',5,NOW()-INTERVAL '2 days',NOW()-INTERVAL '2 days');`,
		)
		require.NoError(t, err, "Expected to insert additional old failed outbox messages without error")

		outboxRepo := repository.NewOutboxRepository(accrualWorker.db)
		count, err := outboxRepo.PurgeOldMessages(context.Background(), 10, 100)
		require.NoError(t, err, "Expected to purge old failed outbox messages without error")
		require.Equal(t, 2, int(count), "Expected two old failed outbox messages to be purged")

		rows, err := accrualWorker.db.QueryContext(context.Background(), `
	SELECT status, attempts, COUNT(*)
	FROM outbox_messages
	GROUP BY status, attempts
	ORDER BY attempts
	`)
		require.NoError(t, err, "Expected to query remaining failed messages without error")
		defer rows.Close()

		remaining := make(map[int]int)
		for rows.Next() {
			var status string
			var attempts int
			var count int
			err := rows.Scan(&status, &attempts, &count)
			require.NoError(t, err, "Expected to scan row without error")
			remaining[attempts] = count
		}
		require.NoError(t, rows.Err(), "Expected no error during row iteration")

		require.Equal(t, 1, remaining[4], "Expected 1 remaining failed message with 4 attempts")
		require.Equal(t, 1, remaining[5], "Expected 1 remaining failed message with 5 attempts")

		var oldTerminalRemaining int
		err = accrualWorker.db.QueryRowContext(context.Background(), `
	SELECT COUNT(*)
	FROM outbox_messages
	WHERE status = 'failed' AND attempts >= 5 AND updated_at < NOW() - INTERVAL '10 days'
	`).Scan(&oldTerminalRemaining)
		require.NoError(t, err, "Expected to query remaining old terminal failed messages without error")
		require.Equal(t, 0, oldTerminalRemaining, "Expected no remaining old terminal failed messages after purge")
	})
}
