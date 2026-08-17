package invoice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/shared"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

const maximumInvoiceItems = 20

var (
	ErrIdempotencyKeyRequired = errors.New("invoice idempotency key is required")
	ErrIdempotencyKeyTooLong  = errors.New("invoice idempotency key is too long")
	ErrTooManyItems           = errors.New("invoice exceeds the maximum number of items")
)

// CreateInputItem is an item requested by the client.
type CreateInputItem struct {
	ProductID string
	Quantity  int64
}

// CreateInput contains the new invoice data.
type CreateInput struct {
	IdempotencyKey string
	Items          []CreateInputItem
}

// Create creates an open invoice with product snapshots.
type Create struct {
	repository invoicerepository.IdempotentCreator
	inventory  inventorygateway.ProductFinder
	ids        shared.IDGenerator
	now        func() time.Time
}

// NewCreate constructs the use case.
func NewCreate(repository invoicerepository.IdempotentCreator, inventory inventorygateway.ProductFinder, ids shared.IDGenerator, now func() time.Time) *Create {
	return &Create{repository: repository, inventory: inventory, ids: ids, now: now}
}

// Execute creates and persists a new invoice.
func (useCase *Create) Execute(ctx context.Context, input CreateInput) (invoiceentity.Invoice, bool, error) {
	idempotencyKey, productIDs, quantities, err := prepareCreateInput(input)
	if err != nil {
		return invoiceentity.Invoice{}, false, err
	}
	requestHash := hashRequest(productIDs, quantities)
	existing, err := useCase.repository.FindByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		if existing.RequestHash != requestHash {
			return invoiceentity.Invoice{}, false, invoicerepository.ErrIdempotencyKeyConflict
		}
		return existing.Invoice, true, nil
	}
	if !errors.Is(err, invoicerepository.ErrNotFound) {
		return invoiceentity.Invoice{}, false, fmt.Errorf("find idempotent invoice: %w", err)
	}

	items := make([]invoiceentity.Item, 0, len(productIDs))
	products, err := useCase.findProducts(ctx, productIDs)
	if err != nil {
		return invoiceentity.Invoice{}, false, err
	}
	for index, productID := range productIDs {
		product := products[index]
		item, err := invoiceentity.NewItem(productID, product.Code, product.Description, quantities[productID])
		if err != nil {
			return invoiceentity.Invoice{}, false, fmt.Errorf("create invoice item: %w", err)
		}
		items = append(items, item)
	}

	created, err := invoiceentity.New(useCase.ids.NewID(), items, useCase.now())
	if err != nil {
		return invoiceentity.Invoice{}, false, err
	}
	persisted, replayed, err := useCase.repository.CreateOrGet(ctx, created, idempotencyKey, requestHash)
	if err != nil {
		return invoiceentity.Invoice{}, false, fmt.Errorf("persist invoice: %w", err)
	}
	return persisted, replayed, nil
}

func prepareCreateInput(input CreateInput) (string, []string, map[string]int64, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return "", nil, nil, ErrIdempotencyKeyRequired
	}
	if len(idempotencyKey) > 200 {
		return "", nil, nil, ErrIdempotencyKeyTooLong
	}
	if len(input.Items) > maximumInvoiceItems {
		return "", nil, nil, ErrTooManyItems
	}

	quantities, err := consolidateQuantities(input.Items)
	if err != nil {
		return "", nil, nil, err
	}
	productIDs := make([]string, 0, len(quantities))
	for productID := range quantities {
		productIDs = append(productIDs, productID)
	}
	sort.Strings(productIDs)
	return idempotencyKey, productIDs, quantities, nil
}

func consolidateQuantities(items []CreateInputItem) (map[string]int64, error) {
	if len(items) == 0 {
		return nil, invoiceentity.ErrItemsRequired
	}
	quantities := make(map[string]int64, len(items))
	for _, requested := range items {
		productID := strings.TrimSpace(requested.ProductID)
		if productID == "" {
			return nil, invoiceentity.ErrInvalidProductID
		}
		if requested.Quantity <= 0 || requested.Quantity > invoiceentity.MaximumSafeInteger {
			return nil, invoiceentity.ErrInvalidQuantity
		}
		if quantities[productID] > invoiceentity.MaximumSafeInteger-requested.Quantity {
			return nil, invoiceentity.ErrInvalidQuantity
		}
		quantities[productID] += requested.Quantity
	}
	return quantities, nil
}

func (useCase *Create) findProducts(ctx context.Context, productIDs []string) ([]inventorygateway.Product, error) {
	products := make([]inventorygateway.Product, len(productIDs))
	errorsByIndex := make([]error, len(productIDs))
	var waitGroup sync.WaitGroup
	for index, productID := range productIDs {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			product, err := useCase.inventory.FindProduct(ctx, productID)
			if err != nil {
				errorsByIndex[index] = fmt.Errorf("find inventory product %s: %w", productID, err)
				return
			}
			products[index] = product
		}()
	}
	waitGroup.Wait()
	for _, err := range errorsByIndex {
		if err != nil {
			return nil, err
		}
	}
	return products, nil
}

func hashRequest(productIDs []string, quantities map[string]int64) string {
	hash := sha256.New()
	for _, productID := range productIDs {
		_, _ = hash.Write([]byte(productID))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strconv.FormatInt(quantities[productID], 10)))
		_, _ = hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
