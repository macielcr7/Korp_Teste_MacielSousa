package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

func TestRepositoryCreateOrGetIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("BILLING_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BILLING_INTEGRATION_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	ctx := context.Background()
	invoiceID := uuid.NewString()
	productID := uuid.NewString()
	key := "create-" + invoiceID
	item, err := invoiceentity.NewItem(productID, "PRD-TEST", "Produto de teste", 2)
	require.NoError(t, err)
	inv, err := invoiceentity.New(invoiceID, []invoiceentity.Item{item}, time.Now())
	require.NoError(t, err)
	repository := New(database)
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM invoices WHERE id=$1`, invoiceID)
	})

	created, replayed, err := repository.CreateOrGet(ctx, inv, key, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	require.False(t, replayed)
	replayCandidate, err := invoiceentity.New(uuid.NewString(), []invoiceentity.Item{item}, time.Now())
	require.NoError(t, err)
	replayedInvoice, replayed, err := repository.CreateOrGet(ctx, replayCandidate, key, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, created.ID(), replayedInvoice.ID())
	_, _, err = repository.CreateOrGet(ctx, replayCandidate, key, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	require.ErrorIs(t, err, invoicerepository.ErrIdempotencyKeyConflict)
}

func TestRepositoryListAgainstPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("BILLING_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BILLING_INTEGRATION_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	status := invoiceentity.StatusClosed

	result, err := New(database).List(context.Background(), invoicerepository.ListCriteria{
		Status: &status, Limit: 10, Offset: 0,
	})

	require.NoError(t, err)
	require.LessOrEqual(t, len(result.Items), 10)
	require.GreaterOrEqual(t, result.Total, int64(len(result.Items)))
	for _, invoice := range result.Items {
		require.Equal(t, invoiceentity.StatusClosed, invoice.Status())
		require.NotEmpty(t, invoice.Items())
	}
}

func TestRepositoryFindDetailReturnsInvoiceAndActiveOperation(t *testing.T) {
	databaseURL := os.Getenv("BILLING_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BILLING_INTEGRATION_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	ctx := context.Background()
	invoiceID := uuid.NewString()
	operationID := uuid.NewString()
	productID := uuid.NewString()
	item, err := invoiceentity.NewItem(productID, "PRD-DETAIL", "Produto detalhado", 2)
	require.NoError(t, err)
	inv, err := invoiceentity.New(invoiceID, []invoiceentity.Item{item}, time.Now())
	require.NoError(t, err)
	repository := New(database)
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM closure_operations WHERE id=$1`, operationID)
		_, _ = database.ExecContext(context.Background(), `DELETE FROM invoices WHERE id=$1`, invoiceID)
	})

	created, _, err := repository.CreateOrGet(ctx, inv, "detail-"+invoiceID, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `
		INSERT INTO closure_operations (
			id, invoice_id, idempotency_key, command_id, status, attempts,
			next_attempt_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'PENDING', 0, NOW(), NOW(), NOW())`,
		operationID, invoiceID, "close-"+invoiceID, uuid.NewString(),
	)
	require.NoError(t, err)

	detail, err := repository.FindDetail(ctx, invoiceID)

	require.NoError(t, err)
	require.Equal(t, created.ID(), detail.Invoice.ID())
	require.NotNil(t, detail.ActiveClosureOperation)
	require.Equal(t, operationID, detail.ActiveClosureOperation.OperationID)
	require.Equal(t, "PENDING", string(detail.ActiveClosureOperation.Status))
}
