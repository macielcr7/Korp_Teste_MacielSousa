// Package closureoperation contains the durable invoice-closing operation.
package closureoperation

import (
	"errors"
	"strings"
	"time"
)

// Status is the technical processing state of a closure operation.
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusRetrying   Status = "RETRYING"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
)

var (
	ErrInvalidOperation  = errors.New("invalid closure operation")
	ErrInvalidTransition = errors.New("invalid closure operation transition")
)

// Operation represents one durable and idempotent invoice-closing intent.
type Operation struct {
	id             string
	invoiceID      string
	idempotencyKey string
	commandID      string
	status         Status
	attempts       int
	nextAttemptAt  time.Time
	leaseUntil     *time.Time
	lastError      string
	createdAt      time.Time
	updatedAt      time.Time
}

// Snapshot contains the persisted state required to rebuild an operation.
type Snapshot struct {
	ID             string
	InvoiceID      string
	IdempotencyKey string
	CommandID      string
	Status         Status
	Attempts       int
	NextAttemptAt  time.Time
	LeaseUntil     *time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// New creates a pending closure operation.
func New(id, invoiceID, idempotencyKey, commandID string, now time.Time) (Operation, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(invoiceID) == "" || strings.TrimSpace(idempotencyKey) == "" || strings.TrimSpace(commandID) == "" {
		return Operation{}, ErrInvalidOperation
	}
	now = now.UTC()
	return Operation{id: id, invoiceID: invoiceID, idempotencyKey: idempotencyKey, commandID: commandID, status: StatusPending, nextAttemptAt: now, createdAt: now, updatedAt: now}, nil
}

// Rehydrate rebuilds an operation stored by a repository.
func Rehydrate(snapshot Snapshot) (Operation, error) {
	if strings.TrimSpace(snapshot.ID) == "" || strings.TrimSpace(snapshot.InvoiceID) == "" || strings.TrimSpace(snapshot.IdempotencyKey) == "" || strings.TrimSpace(snapshot.CommandID) == "" || snapshot.Attempts < 0 {
		return Operation{}, ErrInvalidOperation
	}
	switch snapshot.Status {
	case StatusPending, StatusProcessing, StatusRetrying, StatusCompleted, StatusFailed:
	default:
		return Operation{}, ErrInvalidOperation
	}
	return Operation{
		id: snapshot.ID, invoiceID: snapshot.InvoiceID, idempotencyKey: snapshot.IdempotencyKey,
		commandID: snapshot.CommandID, status: snapshot.Status, attempts: snapshot.Attempts,
		nextAttemptAt: snapshot.NextAttemptAt.UTC(), leaseUntil: cloneTime(snapshot.LeaseUntil),
		lastError: snapshot.LastError, createdAt: snapshot.CreatedAt.UTC(), updatedAt: snapshot.UpdatedAt.UTC(),
	}, nil
}

// MarkRetrying records a retryable technical failure.
func (o *Operation) MarkRetrying(message string, nextAttemptAt, now time.Time) error {
	if o.status != StatusProcessing {
		return ErrInvalidTransition
	}
	o.status = StatusRetrying
	o.lastError = strings.TrimSpace(message)
	o.nextAttemptAt = nextAttemptAt.UTC()
	o.leaseUntil = nil
	o.updatedAt = now.UTC()
	return nil
}

// MarkFailed records a terminal domain failure.
func (o *Operation) MarkFailed(message string, now time.Time) error {
	if o.status != StatusProcessing {
		return ErrInvalidTransition
	}
	o.status = StatusFailed
	o.lastError = strings.TrimSpace(message)
	o.leaseUntil = nil
	o.updatedAt = now.UTC()
	return nil
}

// MarkCompleted records successful stock debit and invoice closure.
func (o *Operation) MarkCompleted(now time.Time) error {
	if o.status != StatusProcessing {
		return ErrInvalidTransition
	}
	o.status = StatusCompleted
	o.lastError = ""
	o.leaseUntil = nil
	o.updatedAt = now.UTC()
	return nil
}

func (o Operation) ID() string               { return o.id }
func (o Operation) InvoiceID() string        { return o.invoiceID }
func (o Operation) IdempotencyKey() string   { return o.idempotencyKey }
func (o Operation) CommandID() string        { return o.commandID }
func (o Operation) Status() Status           { return o.status }
func (o Operation) Attempts() int            { return o.attempts }
func (o Operation) NextAttemptAt() time.Time { return o.nextAttemptAt }
func (o Operation) LeaseUntil() *time.Time   { return cloneTime(o.leaseUntil) }
func (o Operation) LastError() string        { return o.lastError }
func (o Operation) CreatedAt() time.Time     { return o.createdAt }
func (o Operation) UpdatedAt() time.Time     { return o.updatedAt }

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
