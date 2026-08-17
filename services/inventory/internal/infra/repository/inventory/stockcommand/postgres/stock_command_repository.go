// Package postgres implements stock-command persistence with PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

// Repository atomically persists idempotent stock commands in PostgreSQL.
type Repository struct {
	db *sql.DB
}

// New creates a PostgreSQL stock-command repository.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Commit applies every product debit or records a terminal rejection without changing stock.
func (repository *Repository) Commit(ctx context.Context, command commandentity.StockCommand) (result commandrepo.CommitResult, resultErr error) {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return commandrepo.CommitResult{}, fmt.Errorf("begin stock debit: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = tx.Rollback()
		}
	}()

	created, err := insertCommand(ctx, tx, command)
	if err != nil {
		return commandrepo.CommitResult{}, err
	}
	if !created {
		return replayCommand(ctx, tx, command)
	}

	if err := insertCommandItems(ctx, tx, command); err != nil {
		return commandrepo.CommitResult{}, err
	}

	balances, missingProductID, err := lockBalances(ctx, tx, command.Items())
	if errors.Is(err, commandentity.ErrProductNotFound) {
		return repository.reject(ctx, tx, command.CommandID(), commandentity.ErrorProductNotFound,
			fmt.Sprintf("product %s was not found", missingProductID), commandentity.ErrProductNotFound)
	}
	if err != nil {
		return commandrepo.CommitResult{}, err
	}

	changes, err := command.PlanDebits(balances)
	if err != nil {
		var insufficient *commandentity.InsufficientStockError
		if errors.As(err, &insufficient) {
			return repository.reject(ctx, tx, command.CommandID(), commandentity.ErrorInsufficientStock,
				insufficient.Error(), commandentity.ErrInsufficientStock)
		}
		return commandrepo.CommitResult{}, err
	}

	if err := applyDebits(ctx, tx, command, changes); err != nil {
		return commandrepo.CommitResult{}, err
	}
	if err := markCommandCommitted(ctx, tx, command.CommandID()); err != nil {
		return commandrepo.CommitResult{}, err
	}

	committedCommand, err := loadCommand(ctx, tx, command.CommandID())
	if err != nil {
		return commandrepo.CommitResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return commandrepo.CommitResult{}, fmt.Errorf("commit stock debit: %w", err)
	}
	return commandrepo.CommitResult{Command: committedCommand}, nil
}

func replayCommand(ctx context.Context, tx *sql.Tx, command commandentity.StockCommand) (commandrepo.CommitResult, error) {
	existing, err := loadCommand(ctx, tx, command.CommandID())
	if err != nil {
		return commandrepo.CommitResult{}, err
	}
	if existing.PayloadHash() != command.PayloadHash() {
		return commandrepo.CommitResult{}, commandentity.ErrIdempotencyConflict
	}
	if err := tx.Commit(); err != nil {
		return commandrepo.CommitResult{}, fmt.Errorf("commit idempotent stock read: %w", err)
	}
	return commandrepo.CommitResult{Command: existing, Replayed: true}, existing.BusinessError()
}

func insertCommandItems(ctx context.Context, tx *sql.Tx, command commandentity.StockCommand) error {
	for _, item := range command.Items() {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_command_items (command_id, product_id, quantity)
			VALUES ($1, $2, $3)`, command.CommandID(), item.ProductID, item.Quantity); err != nil {
			return fmt.Errorf("insert stock command item: %w", err)
		}
	}
	return nil
}

func lockBalances(ctx context.Context, tx *sql.Tx, items []commandentity.Item) (map[string]int64, string, error) {
	balances := make(map[string]int64, len(items))
	for _, item := range items {
		var balance int64
		err := tx.QueryRowContext(ctx, `SELECT balance FROM products WHERE id = $1 FOR UPDATE`, item.ProductID).Scan(&balance)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, item.ProductID, commandentity.ErrProductNotFound
		}
		if err != nil {
			return nil, "", fmt.Errorf("lock product: %w", err)
		}
		balances[item.ProductID] = balance
	}
	return balances, "", nil
}

func applyDebits(ctx context.Context, tx *sql.Tx, command commandentity.StockCommand, changes []commandentity.BalanceChange) error {
	for _, change := range changes {
		if _, err := tx.ExecContext(ctx, `
			UPDATE products
			SET balance = $2, version = version + 1, updated_at = NOW()
			WHERE id = $1`, change.ProductID, change.BalanceAfter); err != nil {
			return fmt.Errorf("debit product: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO stock_movements
				(id, command_id, invoice_id, product_id, quantity, balance_before, balance_after)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.NewString(), command.CommandID(), command.InvoiceID(), change.ProductID,
			change.Quantity, change.BalanceBefore, change.BalanceAfter,
		); err != nil {
			return fmt.Errorf("insert stock movement: %w", err)
		}
	}
	return nil
}

func markCommandCommitted(ctx context.Context, tx *sql.Tx, commandID string) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE stock_commands SET status = $2, completed_at = NOW() WHERE command_id = $1`,
		commandID, commandentity.StatusCommitted,
	); err != nil {
		return fmt.Errorf("complete stock command: %w", err)
	}
	return nil
}

// GetByID retrieves a durable stock-command result.
func (repository *Repository) GetByID(ctx context.Context, commandID string) (commandentity.StockCommand, error) {
	command, err := loadCommand(ctx, repository.db, commandID)
	if errors.Is(err, sql.ErrNoRows) {
		return commandentity.StockCommand{}, commandentity.ErrNotFound
	}
	if err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("get stock command: %w", err)
	}
	return command, nil
}

func insertCommand(ctx context.Context, tx *sql.Tx, command commandentity.StockCommand) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO stock_commands (command_id, invoice_id, payload_hash, status)
		VALUES ($1, $2, $3, 'PROCESSING')
		ON CONFLICT (command_id) DO NOTHING`, command.CommandID(), command.InvoiceID(), command.PayloadHash())
	if err != nil {
		return false, fmt.Errorf("insert stock command: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect stock command insert: %w", err)
	}
	return rowsAffected == 1, nil
}

func (repository *Repository) reject(
	ctx context.Context,
	tx *sql.Tx,
	commandID, code, detail string,
	businessError error,
) (commandrepo.CommitResult, error) {
	if _, err := tx.ExecContext(ctx, `
		UPDATE stock_commands
		SET status = $2, error_code = $3, error_detail = $4, completed_at = NOW()
		WHERE command_id = $1`, commandID, commandentity.StatusRejected, code, detail); err != nil {
		return commandrepo.CommitResult{}, fmt.Errorf("reject stock command: %w", err)
	}
	result, err := loadCommand(ctx, tx, commandID)
	if err != nil {
		return commandrepo.CommitResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return commandrepo.CommitResult{}, fmt.Errorf("commit rejected stock command: %w", err)
	}
	return commandrepo.CommitResult{Command: result}, businessError
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func loadCommand(ctx context.Context, db queryer, commandID string) (commandentity.StockCommand, error) {
	var snapshot commandentity.Snapshot
	var errorCode sql.NullString
	var errorDetail sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT command_id, invoice_id, payload_hash, status, error_code, error_detail, created_at
		FROM stock_commands
		WHERE command_id = $1`, commandID).Scan(
		&snapshot.CommandID, &snapshot.InvoiceID, &snapshot.PayloadHash, &snapshot.Status,
		&errorCode, &errorDetail, &snapshot.CreatedAt,
	)
	if err != nil {
		return commandentity.StockCommand{}, err
	}
	snapshot.ErrorCode = errorCode.String
	snapshot.ErrorDetail = errorDetail.String

	itemRows, err := db.QueryContext(ctx, `
		SELECT product_id, quantity
		FROM stock_command_items
		WHERE command_id = $1
		ORDER BY product_id`, commandID)
	if err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("query stock command items: %w", err)
	}
	for itemRows.Next() {
		var item commandentity.Item
		if err := itemRows.Scan(&item.ProductID, &item.Quantity); err != nil {
			itemRows.Close()
			return commandentity.StockCommand{}, fmt.Errorf("scan stock command item: %w", err)
		}
		snapshot.Items = append(snapshot.Items, item)
	}
	if err := itemRows.Close(); err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("close stock command items: %w", err)
	}
	if err := itemRows.Err(); err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("iterate stock command items: %w", err)
	}

	movementRows, err := db.QueryContext(ctx, `
		SELECT id, product_id, quantity, balance_before, balance_after, created_at
		FROM stock_movements
		WHERE command_id = $1
		ORDER BY product_id`, commandID)
	if err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("query stock movements: %w", err)
	}
	defer movementRows.Close()
	for movementRows.Next() {
		var movement commandentity.Movement
		if err := movementRows.Scan(&movement.ID, &movement.ProductID, &movement.Quantity,
			&movement.BalanceBefore, &movement.BalanceAfter, &movement.CreatedAt); err != nil {
			return commandentity.StockCommand{}, fmt.Errorf("scan stock movement: %w", err)
		}
		snapshot.Movements = append(snapshot.Movements, movement)
	}
	if err := movementRows.Err(); err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("iterate stock movements: %w", err)
	}
	command, err := commandentity.Rehydrate(snapshot)
	if err != nil {
		return commandentity.StockCommand{}, fmt.Errorf("rehydrate stock command: %w", err)
	}
	return command, nil
}
