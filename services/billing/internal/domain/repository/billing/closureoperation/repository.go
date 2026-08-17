// Package closureoperation defines persistence contracts for closure operations.
package closureoperation

import (
	"context"
	"errors"
	"time"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
)

var (
	ErrNotFound               = errors.New("closure operation not found")
	ErrNoOperationAvailable   = errors.New("no closure operation available")
	ErrIdempotencyKeyConflict = errors.New("idempotency key already used for another invoice")
	ErrActiveOperationExists  = errors.New("invoice already has an active closure operation")
)

// Creator persists an idempotent closure request.
type Creator interface {
	CreateOrGet(ctx context.Context, operation operationentity.Operation) (operationentity.Operation, bool, error)
}

// Finder retrieves a closure operation by identifier.
type Finder interface {
	FindByID(ctx context.Context, id string) (operationentity.Operation, error)
}

// Processor leases and persists transitions of durable closure jobs.
type Processor interface {
	AcquireNext(ctx context.Context, now time.Time, leaseDuration time.Duration) (operationentity.Operation, error)
	MarkRetrying(ctx context.Context, operation operationentity.Operation) error
	MarkFailed(ctx context.Context, operation operationentity.Operation) error
	CompleteWithInvoice(ctx context.Context, operation operationentity.Operation, invoice invoiceentity.Invoice) error
}
