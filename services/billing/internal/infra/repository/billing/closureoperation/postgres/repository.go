// Package postgres implements closure-operation persistence in PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// Repository is a PostgreSQL closure-operation repository.
type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) CreateOrGet(ctx context.Context, operation operationentity.Operation) (operationentity.Operation, bool, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return operationentity.Operation{}, false, fmt.Errorf("begin closure request: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	invoiceStatus, err := lockInvoiceStatus(ctx, tx, operation.InvoiceID())
	if err != nil {
		return operationentity.Operation{}, false, err
	}
	existing, found, err := findByIdempotencyKey(ctx, tx, operation.IdempotencyKey())
	if err != nil {
		return operationentity.Operation{}, false, err
	}
	if found {
		return commitReplay(tx, existing, operation.InvoiceID(), "commit idempotent closure request")
	}
	if invoiceStatus != invoiceentity.StatusOpen {
		return operationentity.Operation{}, false, invoiceentity.ErrInvoiceNotOpen
	}
	if err := ensureNoActiveOperation(ctx, tx, operation.InvoiceID()); err != nil {
		return operationentity.Operation{}, false, err
	}

	created, err := insertOperation(ctx, tx, operation)
	if err != nil {
		return operationentity.Operation{}, false, err
	}
	if !created {
		existing, found, findErr := findByIdempotencyKey(ctx, tx, operation.IdempotencyKey())
		if findErr != nil {
			return operationentity.Operation{}, false, findErr
		}
		if !found {
			return operationentity.Operation{}, false, operationrepository.ErrNotFound
		}
		return commitReplay(tx, existing, operation.InvoiceID(), "commit concurrent idempotent closure request")
	}
	if err := tx.Commit(); err != nil {
		return operationentity.Operation{}, false, fmt.Errorf("commit closure request: %w", err)
	}
	return operation, false, nil
}

func lockInvoiceStatus(ctx context.Context, tx *sql.Tx, invoiceID string) (invoiceentity.Status, error) {
	var status invoiceentity.Status
	err := tx.QueryRowContext(ctx, `SELECT status FROM invoices WHERE id = $1 FOR UPDATE`, invoiceID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", invoicerepository.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock invoice: %w", err)
	}
	return status, nil
}

func findByIdempotencyKey(ctx context.Context, tx *sql.Tx, key string) (operationentity.Operation, bool, error) {
	existing, err := scanOperation(tx.QueryRowContext(ctx, operationSelect+` WHERE idempotency_key = $1`, key))
	if errors.Is(err, operationrepository.ErrNotFound) {
		return operationentity.Operation{}, false, nil
	}
	return existing, err == nil, err
}

func ensureNoActiveOperation(ctx context.Context, tx *sql.Tx, invoiceID string) error {
	var activeID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text FROM closure_operations
		WHERE invoice_id = $1 AND status IN ('PENDING','PROCESSING','RETRYING') LIMIT 1`, invoiceID).Scan(&activeID)
	if err == nil {
		return operationrepository.ErrActiveOperationExists
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return fmt.Errorf("find active closure operation: %w", err)
}

func insertOperation(ctx context.Context, tx *sql.Tx, operation operationentity.Operation) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO closure_operations
			(id, invoice_id, idempotency_key, command_id, status, attempts, next_attempt_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		operation.ID(), operation.InvoiceID(), operation.IdempotencyKey(), operation.CommandID(), operation.Status(), operation.Attempts(), operation.NextAttemptAt(), operation.CreatedAt(), operation.UpdatedAt())
	if err != nil {
		return false, fmt.Errorf("insert closure operation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read insert closure result: %w", err)
	}
	if rows > 1 {
		return false, fmt.Errorf("insert closure operation affected %d rows", rows)
	}
	return rows == 1, nil
}

func commitReplay(tx *sql.Tx, existing operationentity.Operation, invoiceID, action string) (operationentity.Operation, bool, error) {
	if existing.InvoiceID() != invoiceID {
		return operationentity.Operation{}, false, operationrepository.ErrIdempotencyKeyConflict
	}
	if err := tx.Commit(); err != nil {
		return operationentity.Operation{}, false, fmt.Errorf("%s: %w", action, err)
	}
	return existing, true, nil
}

func (repository *Repository) FindByID(ctx context.Context, id string) (operationentity.Operation, error) {
	return scanOperation(repository.db.QueryRowContext(ctx, operationSelect+` WHERE id = $1`, id))
}

func (repository *Repository) FindActiveByInvoiceID(ctx context.Context, invoiceID string) (operationentity.Operation, error) {
	return scanOperation(repository.db.QueryRowContext(ctx, operationSelect+`
		WHERE invoice_id = $1 AND status IN ('PENDING', 'PROCESSING', 'RETRYING')
		LIMIT 1`, invoiceID))
}

func (repository *Repository) AcquireNext(ctx context.Context, now time.Time, leaseDuration time.Duration) (operationentity.Operation, error) {
	leaseUntil := now.Add(leaseDuration)
	row := repository.db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id FROM closure_operations
			WHERE ((status IN ('PENDING','RETRYING') AND next_attempt_at <= $1)
				OR (status = 'PROCESSING' AND lease_until <= $1))
			ORDER BY next_attempt_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE closure_operations AS operation
		SET status = 'PROCESSING', attempts = attempts + 1, lease_until = $2, updated_at = $1
		FROM candidate
		WHERE operation.id = candidate.id
		RETURNING operation.id::text, operation.invoice_id::text, operation.idempotency_key,
			operation.command_id::text, operation.status, operation.attempts, operation.next_attempt_at,
			operation.lease_until, operation.last_error, operation.created_at, operation.updated_at`, now, leaseUntil)
	operation, err := scanOperation(row)
	if errors.Is(err, operationrepository.ErrNotFound) {
		return operationentity.Operation{}, operationrepository.ErrNoOperationAvailable
	}
	return operation, err
}

func (repository *Repository) MarkRetrying(ctx context.Context, operation operationentity.Operation) error {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE closure_operations
		SET status=$2, next_attempt_at=$3, lease_until=NULL, last_error=$4, updated_at=$5
		WHERE id=$1 AND status='PROCESSING' AND attempts=$6`, operation.ID(), operation.Status(), operation.NextAttemptAt(), truncate(operation.LastError()), operation.UpdatedAt(), operation.Attempts())
	return checkUpdated(result, err, "mark closure retrying")
}

func (repository *Repository) MarkFailed(ctx context.Context, operation operationentity.Operation) error {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE closure_operations
		SET status=$2, lease_until=NULL, last_error=$3, updated_at=$4
		WHERE id=$1 AND status='PROCESSING' AND attempts=$5`, operation.ID(), operation.Status(), truncate(operation.LastError()), operation.UpdatedAt(), operation.Attempts())
	return checkUpdated(result, err, "mark closure failed")
}

func (repository *Repository) CompleteWithInvoice(ctx context.Context, operation operationentity.Operation, invoice invoiceentity.Invoice) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin closure completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	invoiceResult, err := tx.ExecContext(ctx, `
		UPDATE invoices SET status=$2, closed_at=$3, version=$4
		WHERE id=$1 AND status='OPEN' AND version=$5`,
		invoice.ID(), invoice.Status(), invoice.ClosedAt(), invoice.Version(), invoice.Version()-1)
	if err != nil {
		return fmt.Errorf("close invoice: %w", err)
	}
	if err := requireOne(invoiceResult, invoiceentity.ErrInvoiceNotOpen); err != nil {
		return err
	}
	operationResult, err := tx.ExecContext(ctx, `
		UPDATE closure_operations
		SET status='COMPLETED', lease_until=NULL, last_error=NULL, updated_at=$2
		WHERE id=$1 AND status='PROCESSING' AND attempts=$3`, operation.ID(), operation.UpdatedAt(), operation.Attempts())
	if err != nil {
		return fmt.Errorf("complete closure operation: %w", err)
	}
	if err := requireOne(operationResult, operationrepository.ErrNotFound); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit closure completion: %w", err)
	}
	return nil
}

const operationSelect = `SELECT id::text, invoice_id::text, idempotency_key, command_id::text,
	status, attempts, next_attempt_at, lease_until, last_error, created_at, updated_at
	FROM closure_operations`

type scanner interface{ Scan(dest ...any) error }

func scanOperation(row scanner) (operationentity.Operation, error) {
	var id, invoiceID, idempotencyKey, commandID string
	var status operationentity.Status
	var attempts int
	var nextAttemptAt, createdAt, updatedAt time.Time
	var leaseUntil sql.NullTime
	var lastError sql.NullString
	err := row.Scan(&id, &invoiceID, &idempotencyKey, &commandID, &status, &attempts, &nextAttemptAt, &leaseUntil, &lastError, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return operationentity.Operation{}, operationrepository.ErrNotFound
	}
	if err != nil {
		return operationentity.Operation{}, fmt.Errorf("scan closure operation: %w", err)
	}
	var lease *time.Time
	if leaseUntil.Valid {
		value := leaseUntil.Time
		lease = &value
	}
	operation, err := operationentity.Rehydrate(operationentity.Snapshot{
		ID: id, InvoiceID: invoiceID, IdempotencyKey: idempotencyKey, CommandID: commandID,
		Status: status, Attempts: attempts, NextAttemptAt: nextAttemptAt, LeaseUntil: lease,
		LastError: lastError.String, CreatedAt: createdAt, UpdatedAt: updatedAt,
	})
	if err != nil {
		return operationentity.Operation{}, fmt.Errorf("rehydrate closure operation: %w", err)
	}
	return operation, nil
}

func checkUpdated(result sql.Result, err error, action string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return requireOne(result, operationrepository.ErrNotFound)
}

func requireOne(result sql.Result, zeroRowsError error) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return zeroRowsError
	}
	return nil
}

func truncate(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}
