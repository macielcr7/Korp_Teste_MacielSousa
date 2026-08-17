package closureoperation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

type operationRepositoryFake struct {
	operation   operationentity.Operation
	retried     bool
	failed      bool
	completed   bool
	invoice     invoiceentity.Invoice
	completeErr error
}

func (repository *operationRepositoryFake) CreateOrGet(context.Context, operationentity.Operation) (operationentity.Operation, bool, error) {
	panic("not used")
}
func (repository *operationRepositoryFake) FindByID(context.Context, string) (operationentity.Operation, error) {
	panic("not used")
}
func (repository *operationRepositoryFake) FindActiveByInvoiceID(context.Context, string) (operationentity.Operation, error) {
	panic("not used")
}
func (repository *operationRepositoryFake) AcquireNext(context.Context, time.Time, time.Duration) (operationentity.Operation, error) {
	return repository.operation, nil
}
func (repository *operationRepositoryFake) MarkRetrying(_ context.Context, operation operationentity.Operation) error {
	repository.retried = true
	repository.operation = operation
	return nil
}
func (repository *operationRepositoryFake) MarkFailed(_ context.Context, operation operationentity.Operation) error {
	repository.failed = true
	repository.operation = operation
	return nil
}
func (repository *operationRepositoryFake) CompleteWithInvoice(_ context.Context, operation operationentity.Operation, invoice invoiceentity.Invoice) error {
	repository.completed = true
	repository.operation = operation
	repository.invoice = invoice
	return repository.completeErr
}

type invoiceRepositoryFake struct{ invoice invoiceentity.Invoice }

func (repository *invoiceRepositoryFake) Create(context.Context, invoiceentity.Invoice) (invoiceentity.Invoice, error) {
	panic("not used")
}
func (repository *invoiceRepositoryFake) FindByID(context.Context, string) (invoiceentity.Invoice, error) {
	return repository.invoice, nil
}
func (repository *invoiceRepositoryFake) List(context.Context, invoicerepository.ListCriteria) (invoicerepository.ListResult, error) {
	panic("not used")
}

type inventoryGatewayFake struct {
	command inventorygateway.DebitCommand
	err     error
}

func (gateway *inventoryGatewayFake) FindProduct(context.Context, string) (inventorygateway.Product, error) {
	panic("not used")
}
func (gateway *inventoryGatewayFake) CommitDebit(_ context.Context, command inventorygateway.DebitCommand) error {
	gateway.command = command
	return gateway.err
}

func processingFixture(t *testing.T) (operationentity.Operation, invoiceentity.Invoice, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	item, err := invoiceentity.NewItem("product-1", "SKU-1", "Keyboard", 2)
	require.NoError(t, err)
	invoice, err := invoiceentity.Rehydrate("invoice-1", 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, now, nil)
	require.NoError(t, err)
	operation, err := operationentity.Rehydrate(operationentity.Snapshot{
		ID: "operation-1", InvoiceID: "invoice-1", IdempotencyKey: "key", CommandID: "command-1",
		Status: operationentity.StatusProcessing, Attempts: 1, NextAttemptAt: now,
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	return operation, invoice, now
}

func TestProcessCompletesOnlyAfterInventorySuccess(t *testing.T) {
	operation, invoice, now := processingFixture(t)
	operations := &operationRepositoryFake{operation: operation}
	inventory := &inventoryGatewayFake{}
	useCase := NewProcess(operations, &invoiceRepositoryFake{invoice: invoice}, inventory, time.Minute, func() time.Time { return now })

	output, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.True(t, output.Processed)
	require.Equal(t, "command-1", output.CommandID)
	require.Equal(t, operationentity.StatusCompleted, output.Status)
	require.True(t, operations.completed)
	require.Equal(t, invoiceentity.StatusClosed, operations.invoice.Status())
	require.Equal(t, "command-1", inventory.command.CommandID)
}

func TestProcessRetriesTechnicalFailure(t *testing.T) {
	operation, invoice, now := processingFixture(t)
	operations := &operationRepositoryFake{operation: operation}
	inventory := &inventoryGatewayFake{err: inventorygateway.ErrUnavailable}
	useCase := NewProcess(operations, &invoiceRepositoryFake{invoice: invoice}, inventory, time.Minute, func() time.Time { return now })

	output, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, operationentity.StatusRetrying, output.Status)
	require.ErrorIs(t, output.Cause, inventorygateway.ErrUnavailable)
	require.Equal(t, retryMessage, operations.operation.LastError())
	require.Equal(t, "command-1", output.CommandID)
	require.True(t, operations.retried)
	require.False(t, operations.completed)
}

func TestProcessFailsTerminalInventoryRejection(t *testing.T) {
	operation, invoice, now := processingFixture(t)
	operations := &operationRepositoryFake{operation: operation}
	inventory := &inventoryGatewayFake{err: &inventorygateway.RejectedError{Code: "INSUFFICIENT_STOCK", Detail: "Insufficient stock."}}
	useCase := NewProcess(operations, &invoiceRepositoryFake{invoice: invoice}, inventory, time.Minute, func() time.Time { return now })

	output, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, operationentity.StatusFailed, output.Status)
	require.Equal(t, "command-1", output.CommandID)
	require.True(t, operations.failed)
	require.False(t, operations.completed)
	require.Equal(t, "Saldo insuficiente para um ou mais produtos da nota.", operations.operation.LastError())
}

func TestProcessDoesNotReportCompletedBeforeTransactionSucceeds(t *testing.T) {
	operation, invoice, now := processingFixture(t)
	operations := &operationRepositoryFake{operation: operation, completeErr: errors.New("database unavailable")}
	useCase := NewProcess(operations, &invoiceRepositoryFake{invoice: invoice}, &inventoryGatewayFake{}, time.Minute, func() time.Time { return now })

	output, err := useCase.Execute(context.Background())

	require.ErrorContains(t, err, "complete closure operation")
	require.Equal(t, operationentity.StatusProcessing, output.Status)
}
