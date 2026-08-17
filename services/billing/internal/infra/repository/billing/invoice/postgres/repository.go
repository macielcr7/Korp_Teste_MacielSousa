// Package postgres implements invoice persistence in PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// Repository is a PostgreSQL invoice repository.
type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (invoicerepository.IdempotentInvoice, error) {
	return findByIdempotencyKey(ctx, repository.db, idempotencyKey)
}

func (repository *Repository) CreateOrGet(ctx context.Context, inv invoiceentity.Invoice, idempotencyKey, requestHash string) (invoiceentity.Invoice, bool, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return invoiceentity.Invoice{}, false, fmt.Errorf("begin create invoice: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var number int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO invoices (id, status, version, created_at, idempotency_key, request_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING number`, inv.ID(), inv.Status(), inv.Version(), inv.CreatedAt(), idempotencyKey, requestHash).Scan(&number)
	if errors.Is(err, sql.ErrNoRows) {
		existing, findErr := findByIdempotencyKey(ctx, tx, idempotencyKey)
		if findErr != nil {
			return invoiceentity.Invoice{}, false, findErr
		}
		if existing.RequestHash != requestHash {
			return invoiceentity.Invoice{}, false, invoicerepository.ErrIdempotencyKeyConflict
		}
		if err := tx.Commit(); err != nil {
			return invoiceentity.Invoice{}, false, fmt.Errorf("commit idempotent invoice read: %w", err)
		}
		return existing.Invoice, true, nil
	}
	if err != nil {
		return invoiceentity.Invoice{}, false, fmt.Errorf("insert invoice: %w", err)
	}
	for _, item := range inv.Items() {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO invoice_items (invoice_id, product_id, product_code, product_description, quantity)
			VALUES ($1, $2, $3, $4, $5)`, inv.ID(), item.ProductID(), item.Code(), item.Description(), item.Quantity())
		if err != nil {
			return invoiceentity.Invoice{}, false, fmt.Errorf("insert invoice item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return invoiceentity.Invoice{}, false, fmt.Errorf("commit create invoice: %w", err)
	}
	persisted, err := invoiceentity.Rehydrate(inv.ID(), number, inv.Status(), inv.Items(), inv.Version(), inv.CreatedAt(), inv.ClosedAt())
	if err != nil {
		return invoiceentity.Invoice{}, false, fmt.Errorf("rehydrate created invoice: %w", err)
	}
	return persisted, false, nil
}

func (repository *Repository) FindByID(ctx context.Context, id string) (invoiceentity.Invoice, error) {
	row := repository.db.QueryRowContext(ctx, `
		SELECT id::text, number, status, version, created_at, closed_at
		FROM invoices WHERE id = $1`, id)
	return scanInvoice(ctx, repository.db, row)
}

func (repository *Repository) FindDetail(ctx context.Context, id string) (invoicerepository.Detail, error) {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return invoicerepository.Detail{}, fmt.Errorf("begin invoice detail snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id::text, number, status, version, created_at, closed_at
		FROM invoices WHERE id = $1`, id)
	inv, err := scanInvoice(ctx, tx, row)
	if err != nil {
		return invoicerepository.Detail{}, err
	}

	var operationID string
	var status operationentity.Status
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, status
		FROM closure_operations
		WHERE invoice_id = $1 AND status IN ('PENDING', 'PROCESSING', 'RETRYING')
		LIMIT 1`, id).Scan(&operationID, &status)
	detail := invoicerepository.Detail{Invoice: inv}
	if err == nil {
		detail.ActiveClosureOperation = &invoicerepository.ActiveClosureOperation{
			OperationID: operationID,
			Status:      status,
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return invoicerepository.Detail{}, fmt.Errorf("query active closure operation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return invoicerepository.Detail{}, fmt.Errorf("commit invoice detail snapshot: %w", err)
	}
	return detail, nil
}

func (repository *Repository) List(ctx context.Context, criteria invoicerepository.ListCriteria) (invoicerepository.ListResult, error) {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return invoicerepository.ListResult{}, fmt.Errorf("begin invoice list snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	where, filterArguments := buildListFilter(criteria)
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices`+where, filterArguments...).Scan(&total); err != nil {
		return invoicerepository.ListResult{}, fmt.Errorf("count invoices: %w", err)
	}

	query := `
		SELECT page.id::text, page.number, page.status, page.version, page.created_at, page.closed_at,
		       item.product_id::text, item.product_code, item.product_description, item.quantity
		FROM (
			SELECT id, number, status, version, created_at, closed_at
			FROM invoices` + where + `
			ORDER BY number DESC` + fmt.Sprintf(
		"\n\t\t\tLIMIT $%d OFFSET $%d", len(filterArguments)+1, len(filterArguments)+2,
	) + `
		) AS page
		JOIN invoice_items AS item ON item.invoice_id = page.id
		ORDER BY page.number DESC, item.product_code, item.product_id`
	arguments := append(filterArguments, criteria.Limit, criteria.Offset)
	rows, err := tx.QueryContext(ctx, query, arguments...)
	if err != nil {
		return invoicerepository.ListResult{}, fmt.Errorf("query invoices: %w", err)
	}
	defer rows.Close()

	headers, err := scanInvoiceHeaders(rows, criteria.Limit)
	if err != nil {
		return invoicerepository.ListResult{}, err
	}
	if err := rows.Close(); err != nil {
		return invoicerepository.ListResult{}, fmt.Errorf("close invoice page: %w", err)
	}
	result, err := rehydrateInvoiceHeaders(headers)
	if err != nil {
		return invoicerepository.ListResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return invoicerepository.ListResult{}, fmt.Errorf("commit invoice list snapshot: %w", err)
	}
	return invoicerepository.ListResult{Items: result, Total: total}, nil
}

type invoiceHeader struct {
	id        string
	number    int64
	status    invoiceentity.Status
	version   int64
	createdAt sql.NullTime
	closedAt  sql.NullTime
	items     []invoiceentity.Item
}

func scanInvoiceHeaders(rows *sql.Rows, capacity int) ([]invoiceHeader, error) {
	headers := make([]invoiceHeader, 0, capacity)
	for rows.Next() {
		var itemHeader invoiceHeader
		var productID, code, description string
		var quantity int64
		if err := rows.Scan(
			&itemHeader.id, &itemHeader.number, &itemHeader.status, &itemHeader.version,
			&itemHeader.createdAt, &itemHeader.closedAt, &productID, &code, &description, &quantity,
		); err != nil {
			return nil, fmt.Errorf("scan invoice page: %w", err)
		}
		item, err := invoiceentity.NewItem(productID, code, description, quantity)
		if err != nil {
			return nil, fmt.Errorf("rehydrate listed invoice item: %w", err)
		}
		if len(headers) == 0 || headers[len(headers)-1].id != itemHeader.id {
			itemHeader.items = []invoiceentity.Item{item}
			headers = append(headers, itemHeader)
		} else {
			headers[len(headers)-1].items = append(headers[len(headers)-1].items, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoices: %w", err)
	}
	return headers, nil
}

func rehydrateInvoiceHeaders(headers []invoiceHeader) ([]invoiceentity.Invoice, error) {
	result := make([]invoiceentity.Invoice, 0, len(headers))
	for _, header := range headers {
		createdAt := header.createdAt.Time
		var closedAt *sql.NullTime
		if header.closedAt.Valid {
			closedAt = &header.closedAt
		}
		inv, err := rehydrate(header.id, header.number, header.status, header.version, createdAt, closedAt, header.items)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, nil
}

func buildListFilter(criteria invoicerepository.ListCriteria) (string, []any) {
	if criteria.Status == nil {
		return "", nil
	}
	return " WHERE status = $1", []any{*criteria.Status}
}

type scanner interface{ Scan(dest ...any) error }

func scanInvoice(ctx context.Context, database queryer, row scanner) (invoiceentity.Invoice, error) {
	var id string
	var number, version int64
	var status invoiceentity.Status
	var createdAt sql.NullTime
	var closedAt sql.NullTime
	if err := row.Scan(&id, &number, &status, &version, &createdAt, &closedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return invoiceentity.Invoice{}, invoicerepository.ErrNotFound
		}
		return invoiceentity.Invoice{}, fmt.Errorf("scan invoice: %w", err)
	}
	items, err := loadItems(ctx, database, id)
	if err != nil {
		return invoiceentity.Invoice{}, err
	}
	var nullableClosedAt *sql.NullTime
	if closedAt.Valid {
		nullableClosedAt = &closedAt
	}
	return rehydrate(id, number, status, version, createdAt.Time, nullableClosedAt, items)
}

func loadItems(ctx context.Context, database queryer, invoiceID string) ([]invoiceentity.Item, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT product_id::text, product_code, product_description, quantity
		FROM invoice_items WHERE invoice_id = $1 ORDER BY product_code, product_id`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("query invoice items: %w", err)
	}
	defer rows.Close()
	items := make([]invoiceentity.Item, 0)
	for rows.Next() {
		var productID, code, description string
		var quantity int64
		if err := rows.Scan(&productID, &code, &description, &quantity); err != nil {
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		item, err := invoiceentity.NewItem(productID, code, description, quantity)
		if err != nil {
			return nil, fmt.Errorf("rehydrate invoice item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoice items: %w", err)
	}
	return items, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findByIdempotencyKey(ctx context.Context, database queryer, idempotencyKey string) (invoicerepository.IdempotentInvoice, error) {
	var id, requestHash string
	var number, version int64
	var status invoiceentity.Status
	var createdAt, closedAt sql.NullTime
	err := database.QueryRowContext(ctx, `
		SELECT id::text, number, status, version, created_at, closed_at, request_hash
		FROM invoices WHERE idempotency_key = $1`, idempotencyKey).Scan(
		&id, &number, &status, &version, &createdAt, &closedAt, &requestHash,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return invoicerepository.IdempotentInvoice{}, invoicerepository.ErrNotFound
	}
	if err != nil {
		return invoicerepository.IdempotentInvoice{}, fmt.Errorf("find invoice by idempotency key: %w", err)
	}
	items, err := loadItems(ctx, database, id)
	if err != nil {
		return invoicerepository.IdempotentInvoice{}, err
	}
	var nullableClosedAt *sql.NullTime
	if closedAt.Valid {
		nullableClosedAt = &closedAt
	}
	inv, err := rehydrate(id, number, status, version, createdAt.Time, nullableClosedAt, items)
	if err != nil {
		return invoicerepository.IdempotentInvoice{}, err
	}
	return invoicerepository.IdempotentInvoice{Invoice: inv, RequestHash: requestHash}, nil
}

func rehydrate(id string, number int64, status invoiceentity.Status, version int64, createdAt time.Time, nullableClosedAt *sql.NullTime, items []invoiceentity.Item) (invoiceentity.Invoice, error) {
	var closedAt *time.Time
	if nullableClosedAt != nil && nullableClosedAt.Valid {
		value := nullableClosedAt.Time
		closedAt = &value
	}
	inv, err := invoiceentity.Rehydrate(id, number, status, items, version, createdAt, closedAt)
	if err != nil {
		return invoiceentity.Invoice{}, fmt.Errorf("rehydrate invoice: %w", err)
	}
	return inv, nil
}
