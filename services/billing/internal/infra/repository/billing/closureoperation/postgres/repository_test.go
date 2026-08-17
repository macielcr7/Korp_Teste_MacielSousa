package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
)

func TestRepositoryRejectsStaleLeaseOwner(t *testing.T) {
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
	commandID := uuid.NewString()
	baseTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = database.ExecContext(ctx, `
		INSERT INTO invoices (id, status, version, created_at)
		VALUES ($1, 'OPEN', 1, $2)`, invoiceID, baseTime)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM closure_operations WHERE id=$1`, operationID)
		_, _ = database.ExecContext(context.Background(), `DELETE FROM invoices WHERE id=$1`, invoiceID)
	})
	operation, err := operationentity.New(operationID, invoiceID, "lease-"+operationID, commandID, baseTime)
	require.NoError(t, err)
	repository := New(database)
	_, _, err = repository.CreateOrGet(ctx, operation)
	require.NoError(t, err)

	firstLease, err := repository.AcquireNext(ctx, baseTime, time.Second)
	require.NoError(t, err)
	secondLease, err := repository.AcquireNext(ctx, baseTime.Add(2*time.Second), time.Second)
	require.NoError(t, err)
	require.Equal(t, firstLease.ID(), secondLease.ID())
	require.Equal(t, firstLease.Attempts()+1, secondLease.Attempts())
	require.NoError(t, firstLease.MarkRetrying("safe", baseTime.Add(time.Minute), baseTime.Add(2*time.Second)))

	err = repository.MarkRetrying(ctx, firstLease)

	require.True(t, errors.Is(err, operationrepository.ErrNotFound), err)
}

func TestRepositoryStaleLeaseCannotCloseInvoice(t *testing.T) {
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
	baseTime := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = database.ExecContext(ctx, `
		INSERT INTO invoices (id, status, version, created_at)
		VALUES ($1, 'OPEN', 1, $2)`, invoiceID, baseTime)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM closure_operations WHERE id=$1`, operationID)
		_, _ = database.ExecContext(context.Background(), `DELETE FROM invoices WHERE id=$1`, invoiceID)
	})
	operation, err := operationentity.New(operationID, invoiceID, "lease-"+operationID, uuid.NewString(), baseTime)
	require.NoError(t, err)
	repository := New(database)
	_, _, err = repository.CreateOrGet(ctx, operation)
	require.NoError(t, err)
	staleLease, err := repository.AcquireNext(ctx, baseTime, time.Second)
	require.NoError(t, err)
	_, err = repository.AcquireNext(ctx, baseTime.Add(2*time.Second), time.Second)
	require.NoError(t, err)
	require.NoError(t, staleLease.MarkCompleted(baseTime.Add(2*time.Second)))
	item, err := invoiceentity.NewItem(uuid.NewString(), "SKU", "Product", 1)
	require.NoError(t, err)
	invoice, err := invoiceentity.Rehydrate(invoiceID, 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, baseTime, nil)
	require.NoError(t, err)
	require.NoError(t, invoice.Close(baseTime.Add(2*time.Second)))

	err = repository.CompleteWithInvoice(ctx, staleLease, invoice)

	require.True(t, errors.Is(err, operationrepository.ErrNotFound), err)
	var status string
	require.NoError(t, database.QueryRowContext(ctx, `SELECT status FROM invoices WHERE id=$1`, invoiceID).Scan(&status))
	require.Equal(t, "OPEN", status)
}
