// Package invoice defines persistence contracts for invoices.
package invoice

import (
	"context"
	"errors"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
)

var (
	ErrNotFound               = errors.New("invoice not found")
	ErrIdempotencyKeyConflict = errors.New("invoice idempotency key already used with different data")
)

// IdempotentInvoice is an invoice previously associated with a request hash.
type IdempotentInvoice struct {
	Invoice     invoiceentity.Invoice
	RequestHash string
}

// ListCriteria contains persistence-neutral invoice filters and pagination.
type ListCriteria struct {
	Status *invoiceentity.Status
	Limit  int
	Offset int
}

// ListResult contains one invoice page and the total matching rows.
type ListResult struct {
	Items []invoiceentity.Invoice
	Total int64
}

// ActiveClosureOperation contains the polling state associated with an invoice snapshot.
type ActiveClosureOperation struct {
	OperationID string
	Status      operationentity.Status
}

// Detail combines an invoice and its active closure operation from one consistent read.
type Detail struct {
	Invoice                invoiceentity.Invoice
	ActiveClosureOperation *ActiveClosureOperation
}

// IdempotentCreator creates invoices with request-level idempotency.
type IdempotentCreator interface {
	FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (IdempotentInvoice, error)
	CreateOrGet(ctx context.Context, invoice invoiceentity.Invoice, idempotencyKey, requestHash string) (invoiceentity.Invoice, bool, error)
}

// Finder retrieves invoices by identifier.
type Finder interface {
	FindByID(ctx context.Context, id string) (invoiceentity.Invoice, error)
}

// DetailFinder retrieves an invoice and its active operation from one consistent snapshot.
type DetailFinder interface {
	FindDetail(ctx context.Context, id string) (Detail, error)
}

// Lister retrieves filtered invoice pages.
type Lister interface {
	List(ctx context.Context, criteria ListCriteria) (ListResult, error)
}
