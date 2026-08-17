// Package invoice contains the billing invoice aggregate.
package invoice

import (
	"errors"
	"strings"
	"time"
)

// Status is the public lifecycle state of an invoice.
type Status string

const (
	StatusOpen   Status = "OPEN"
	StatusClosed Status = "CLOSED"
)

// MaximumSafeInteger is the largest exact integer supported by JSON clients based on IEEE-754.
const MaximumSafeInteger int64 = 9_007_199_254_740_991

var (
	ErrInvalidID          = errors.New("invoice id is required")
	ErrItemsRequired      = errors.New("invoice requires at least one item")
	ErrInvalidProductID   = errors.New("product id is required")
	ErrInvalidProductCode = errors.New("product code is required")
	ErrInvalidDescription = errors.New("product description is required")
	ErrInvalidQuantity    = errors.New("quantity must be a positive safe integer")
	ErrDuplicateProduct   = errors.New("invoice cannot contain duplicate products")
	ErrInvoiceNotOpen     = errors.New("invoice is not open")
	ErrInvoiceNotClosed   = errors.New("invoice is not closed")
)

// Item is an immutable product snapshot in an invoice.
type Item struct {
	productID   string
	code        string
	description string
	quantity    int64
}

// NewItem validates and creates an invoice item.
func NewItem(productID, code, description string, quantity int64) (Item, error) {
	productID = strings.TrimSpace(productID)
	code = strings.TrimSpace(code)
	description = strings.TrimSpace(description)
	switch {
	case productID == "":
		return Item{}, ErrInvalidProductID
	case code == "":
		return Item{}, ErrInvalidProductCode
	case description == "":
		return Item{}, ErrInvalidDescription
	case quantity <= 0 || quantity > MaximumSafeInteger:
		return Item{}, ErrInvalidQuantity
	}
	return Item{productID: productID, code: code, description: description, quantity: quantity}, nil
}

func (i Item) ProductID() string   { return i.productID }
func (i Item) Code() string        { return i.code }
func (i Item) Description() string { return i.description }
func (i Item) Quantity() int64     { return i.quantity }

// Invoice is the aggregate root for a simplified billing note.
type Invoice struct {
	id        string
	number    int64
	status    Status
	items     []Item
	version   int64
	createdAt time.Time
	closedAt  *time.Time
}

// New creates a valid open invoice.
func New(id string, items []Item, now time.Time) (Invoice, error) {
	if strings.TrimSpace(id) == "" {
		return Invoice{}, ErrInvalidID
	}
	if err := validateItems(items); err != nil {
		return Invoice{}, err
	}
	return Invoice{id: id, status: StatusOpen, items: cloneItems(items), version: 1, createdAt: now.UTC()}, nil
}

// Rehydrate rebuilds an invoice stored by a repository.
func Rehydrate(id string, number int64, status Status, items []Item, version int64, createdAt time.Time, closedAt *time.Time) (Invoice, error) {
	if strings.TrimSpace(id) == "" || number <= 0 || number > MaximumSafeInteger || version <= 0 || version > MaximumSafeInteger {
		return Invoice{}, errors.New("invalid persisted invoice")
	}
	if status != StatusOpen && status != StatusClosed {
		return Invoice{}, errors.New("invalid persisted invoice status")
	}
	if err := validateItems(items); err != nil {
		return Invoice{}, err
	}
	if status == StatusClosed && closedAt == nil {
		return Invoice{}, errors.New("closed invoice requires closed timestamp")
	}
	return Invoice{id: id, number: number, status: status, items: cloneItems(items), version: version, createdAt: createdAt.UTC(), closedAt: cloneTime(closedAt)}, nil
}

// Close transitions an open invoice to closed.
func (i *Invoice) Close(now time.Time) error {
	if i.status != StatusOpen {
		return ErrInvoiceNotOpen
	}
	closedAt := now.UTC()
	i.status = StatusClosed
	i.closedAt = &closedAt
	i.version++
	return nil
}

func (i Invoice) ID() string           { return i.id }
func (i Invoice) Number() int64        { return i.number }
func (i Invoice) Status() Status       { return i.status }
func (i Invoice) Items() []Item        { return cloneItems(i.items) }
func (i Invoice) Version() int64       { return i.version }
func (i Invoice) CreatedAt() time.Time { return i.createdAt }
func (i Invoice) ClosedAt() *time.Time { return cloneTime(i.closedAt) }

func validateItems(items []Item) error {
	if len(items) == 0 {
		return ErrItemsRequired
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.productID == "" || item.code == "" || item.description == "" || item.quantity <= 0 || item.quantity > MaximumSafeInteger {
			return errors.New("invalid invoice item")
		}
		if _, exists := seen[item.productID]; exists {
			return ErrDuplicateProduct
		}
		seen[item.productID] = struct{}{}
	}
	return nil
}

func cloneItems(items []Item) []Item {
	return append([]Item(nil), items...)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
