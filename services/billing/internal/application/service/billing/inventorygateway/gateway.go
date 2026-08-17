// Package inventorygateway defines the technical port used to access Inventory.
package inventorygateway

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrUnavailable     = errors.New("inventory service unavailable")
	ErrProductNotFound = errors.New("inventory product not found")
)

// Product is the product snapshot needed by Billing.
type Product struct {
	ID          string
	Code        string
	Description string
}

// DebitItem is an item sent to the stock-debit command.
type DebitItem struct {
	ProductID string
	Quantity  int64
}

// DebitCommand is the idempotent stock-debit contract.
type DebitCommand struct {
	CommandID string
	InvoiceID string
	Items     []DebitItem
}

// RejectedError represents a terminal business rejection from Inventory.
type RejectedError struct {
	Code   string
	Detail string
}

func (e *RejectedError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("inventory rejected debit: %s", e.Code)
	}
	return e.Detail
}

// ProductFinder retrieves the product snapshot needed to create an invoice.
type ProductFinder interface {
	FindProduct(ctx context.Context, productID string) (Product, error)
}

// DebitCommitter atomically submits an idempotent stock debit.
type DebitCommitter interface {
	CommitDebit(ctx context.Context, command DebitCommand) error
}
