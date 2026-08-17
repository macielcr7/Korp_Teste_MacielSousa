package invoice

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

type fakeIDGenerator struct{ values []string }

func (generator *fakeIDGenerator) NewID() string {
	value := generator.values[0]
	generator.values = generator.values[1:]
	return value
}

type fakeInventory struct {
	products map[string]inventorygateway.Product
	debit    inventorygateway.DebitCommand
	err      error
	finds    int
}

func (inventory *fakeInventory) FindProduct(_ context.Context, id string) (inventorygateway.Product, error) {
	inventory.finds++
	product, found := inventory.products[id]
	if !found {
		return inventorygateway.Product{}, inventorygateway.ErrProductNotFound
	}
	return product, nil
}

func (inventory *fakeInventory) CommitDebit(_ context.Context, command inventorygateway.DebitCommand) error {
	inventory.debit = command
	return inventory.err
}

type fakeInvoiceRepository struct {
	invoice     invoiceentity.Invoice
	requestHash string
}

func (repository *fakeInvoiceRepository) FindByIdempotencyKey(_ context.Context, _ string) (invoicerepository.IdempotentInvoice, error) {
	if repository.invoice.ID() == "" {
		return invoicerepository.IdempotentInvoice{}, invoicerepository.ErrNotFound
	}
	return invoicerepository.IdempotentInvoice{Invoice: repository.invoice, RequestHash: repository.requestHash}, nil
}
func (repository *fakeInvoiceRepository) CreateOrGet(_ context.Context, invoice invoiceentity.Invoice, _ string, requestHash string) (invoiceentity.Invoice, bool, error) {
	repository.invoice = invoice
	repository.requestHash = requestHash
	persisted, err := invoiceentity.Rehydrate(invoice.ID(), 1, invoice.Status(), invoice.Items(), invoice.Version(), invoice.CreatedAt(), nil)
	return persisted, false, err
}
func (repository *fakeInvoiceRepository) FindByID(_ context.Context, _ string) (invoiceentity.Invoice, error) {
	return repository.invoice, nil
}
func (repository *fakeInvoiceRepository) List(_ context.Context, _ invoicerepository.ListCriteria) (invoicerepository.ListResult, error) {
	return invoicerepository.ListResult{Items: []invoiceentity.Invoice{repository.invoice}, Total: 1}, nil
}

func TestCreateConsolidatesProductsAndCapturesSnapshots(t *testing.T) {
	inventory := &fakeInventory{products: map[string]inventorygateway.Product{
		"b329cad1-9dc0-4536-99af-83d7bd8f7a29": {ID: "b329cad1-9dc0-4536-99af-83d7bd8f7a29", Code: "SKU-1", Description: "Keyboard"},
	}}
	repository := &fakeInvoiceRepository{}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	useCase := NewCreate(repository, inventory, &fakeIDGenerator{values: []string{"invoice-1"}}, func() time.Time { return now })

	created, replayed, err := useCase.Execute(context.Background(), CreateInput{IdempotencyKey: "request-1", Items: []CreateInputItem{{ProductID: "b329cad1-9dc0-4536-99af-83d7bd8f7a29", Quantity: 2}, {ProductID: "b329cad1-9dc0-4536-99af-83d7bd8f7a29", Quantity: 3}}})

	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, int64(1), created.Number())
	require.Len(t, created.Items(), 1)
	require.Equal(t, int64(5), created.Items()[0].Quantity())
	require.Equal(t, "Keyboard", created.Items()[0].Description())
}

func TestCreateReplaysExistingInvoiceWithoutCallingInventory(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	productID := "b329cad1-9dc0-4536-99af-83d7bd8f7a29"
	item, err := invoiceentity.NewItem(productID, "SKU-1", "Keyboard", 2)
	require.NoError(t, err)
	existing, err := invoiceentity.Rehydrate("4fd74eea-80c3-4ef0-934d-7cf83166929d", 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, now, nil)
	require.NoError(t, err)
	quantities := map[string]int64{productID: 2}
	repository := &fakeInvoiceRepository{invoice: existing, requestHash: hashRequest([]string{productID}, quantities)}
	inventory := &fakeInventory{}
	useCase := NewCreate(repository, inventory, &fakeIDGenerator{}, func() time.Time { return now })

	created, replayed, err := useCase.Execute(context.Background(), CreateInput{IdempotencyKey: "request-1", Items: []CreateInputItem{{ProductID: productID, Quantity: 2}}})

	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, existing.ID(), created.ID())
	require.Zero(t, inventory.finds)
}

func TestCreateRejectsIdempotencyKeyReusedWithDifferentItems(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	productID := "b329cad1-9dc0-4536-99af-83d7bd8f7a29"
	item, err := invoiceentity.NewItem(productID, "SKU-1", "Keyboard", 1)
	require.NoError(t, err)
	existing, err := invoiceentity.Rehydrate("4fd74eea-80c3-4ef0-934d-7cf83166929d", 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, now, nil)
	require.NoError(t, err)
	repository := &fakeInvoiceRepository{invoice: existing, requestHash: hashRequest([]string{productID}, map[string]int64{productID: 1})}
	useCase := NewCreate(repository, &fakeInventory{}, &fakeIDGenerator{}, func() time.Time { return now })

	_, _, err = useCase.Execute(context.Background(), CreateInput{IdempotencyKey: "request-1", Items: []CreateInputItem{{ProductID: productID, Quantity: 2}}})

	require.ErrorIs(t, err, invoicerepository.ErrIdempotencyKeyConflict)
}

func TestCreateRejectsMoreThanMaximumItems(t *testing.T) {
	items := make([]CreateInputItem, maximumInvoiceItems+1)
	useCase := NewCreate(&fakeInvoiceRepository{}, &fakeInventory{}, &fakeIDGenerator{}, time.Now)

	_, _, err := useCase.Execute(context.Background(), CreateInput{IdempotencyKey: "request-1", Items: items})

	require.ErrorIs(t, err, ErrTooManyItems)
}

func TestCreateRejectsConsolidatedQuantityAboveJSONSafeInteger(t *testing.T) {
	productID := "b329cad1-9dc0-4536-99af-83d7bd8f7a29"
	useCase := NewCreate(&fakeInvoiceRepository{}, &fakeInventory{}, &fakeIDGenerator{}, time.Now)

	_, _, err := useCase.Execute(context.Background(), CreateInput{
		IdempotencyKey: "request-1",
		Items: []CreateInputItem{
			{ProductID: productID, Quantity: invoiceentity.MaximumSafeInteger},
			{ProductID: productID, Quantity: 1},
		},
	})

	require.ErrorIs(t, err, invoiceentity.ErrInvalidQuantity)
}

type blockingInventory struct {
	mutex   sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (inventory *blockingInventory) FindProduct(_ context.Context, id string) (inventorygateway.Product, error) {
	inventory.mutex.Lock()
	inventory.calls++
	if inventory.calls == 2 {
		close(inventory.started)
	}
	inventory.mutex.Unlock()
	<-inventory.release
	return inventorygateway.Product{ID: id, Code: id, Description: id}, nil
}

func (*blockingInventory) CommitDebit(context.Context, inventorygateway.DebitCommand) error {
	return nil
}

func TestCreateLoadsProductSnapshotsConcurrently(t *testing.T) {
	firstID := "b329cad1-9dc0-4536-99af-83d7bd8f7a29"
	secondID := "4fd74eea-80c3-4ef0-934d-7cf83166929d"
	inventory := &blockingInventory{started: make(chan struct{}), release: make(chan struct{})}
	useCase := NewCreate(&fakeInvoiceRepository{}, inventory, &fakeIDGenerator{values: []string{"invoice-1"}}, time.Now)
	finished := make(chan error, 1)
	go func() {
		_, _, err := useCase.Execute(context.Background(), CreateInput{IdempotencyKey: "request-1", Items: []CreateInputItem{{ProductID: firstID, Quantity: 1}, {ProductID: secondID, Quantity: 1}}})
		finished <- err
	}()

	select {
	case <-inventory.started:
	case <-time.After(time.Second):
		t.Fatal("product lookups did not run concurrently")
	}
	close(inventory.release)
	require.NoError(t, <-finished)
}
