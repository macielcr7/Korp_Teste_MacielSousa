// Package postgres implements product persistence with PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

// Repository persists inventory products in PostgreSQL.
type Repository struct {
	db *sql.DB
}

// New creates a PostgreSQL product repository.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create persists a product.
func (repository *Repository) Create(ctx context.Context, product productentity.Product) error {
	const query = `
		INSERT INTO products (id, code, description, balance, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := repository.db.ExecContext(ctx, query,
		product.ID(), product.Code(), product.Description(), product.Balance(),
		product.Version(), product.CreatedAt(), product.UpdatedAt(),
	)
	if err == nil {
		return nil
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return productentity.ErrDuplicateCode
	}
	return fmt.Errorf("create product: %w", err)
}

// GetByID retrieves a product by its identifier.
func (repository *Repository) GetByID(ctx context.Context, id string) (productentity.Product, error) {
	const query = `
		SELECT id, code, description, balance, version, created_at, updated_at
		FROM products
		WHERE id = $1`

	product, err := scanProduct(repository.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return productentity.Product{}, productentity.ErrNotFound
	}
	if err != nil {
		return productentity.Product{}, fmt.Errorf("get product: %w", err)
	}
	return product, nil
}

// List retrieves a stable filtered page of products.
func (repository *Repository) List(ctx context.Context, criteria productrepo.ListCriteria) (productrepo.ListResult, error) {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return productrepo.ListResult{}, fmt.Errorf("begin product list snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	where, filterArguments := buildListFilter(criteria)
	countQuery := `SELECT COUNT(*) FROM products` + where

	var total int64
	if err := tx.QueryRowContext(ctx, countQuery, filterArguments...).Scan(&total); err != nil {
		return productrepo.ListResult{}, fmt.Errorf("count products: %w", err)
	}

	query := `
		SELECT id, code, description, balance, version, created_at, updated_at
		FROM products` + where + `
		ORDER BY code, id` + fmt.Sprintf(
		"\n\t\tLIMIT $%d OFFSET $%d", len(filterArguments)+1, len(filterArguments)+2,
	)
	arguments := append(filterArguments, criteria.Limit, criteria.Offset)

	rows, err := tx.QueryContext(ctx, query, arguments...)
	if err != nil {
		return productrepo.ListResult{}, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]productentity.Product, 0)
	for rows.Next() {
		product, scanErr := scanProduct(rows)
		if scanErr != nil {
			return productrepo.ListResult{}, fmt.Errorf("scan product: %w", scanErr)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return productrepo.ListResult{}, fmt.Errorf("iterate products: %w", err)
	}
	if err := rows.Close(); err != nil {
		return productrepo.ListResult{}, fmt.Errorf("close products: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return productrepo.ListResult{}, fmt.Errorf("commit product list snapshot: %w", err)
	}
	return productrepo.ListResult{Items: products, Total: total}, nil
}

func buildListFilter(criteria productrepo.ListCriteria) (string, []any) {
	clauses := make([]string, 0, 3)
	arguments := make([]any, 0, 3)
	if criteria.Search != "" {
		arguments = append(arguments, escapeLikePattern(criteria.Search))
		position := len(arguments)
		clauses = append(clauses, fmt.Sprintf(
			`(lower(code) LIKE '%%' || lower($%d::text) || '%%' ESCAPE '\' OR lower(description) LIKE '%%' || lower($%d::text) || '%%' ESCAPE '\')`,
			position, position,
		))
	}
	if criteria.MinimumBalance != nil {
		arguments = append(arguments, *criteria.MinimumBalance)
		clauses = append(clauses, fmt.Sprintf("balance >= $%d", len(arguments)))
	}
	if criteria.MaximumBalance != nil {
		arguments = append(arguments, *criteria.MaximumBalance)
		clauses = append(clauses, fmt.Sprintf("balance <= $%d", len(arguments)))
	}
	if len(clauses) == 0 {
		return "", arguments
	}
	return " WHERE " + strings.Join(clauses, " AND "), arguments
}

func escapeLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

type productScanner interface {
	Scan(dest ...any) error
}

func scanProduct(scanner productScanner) (productentity.Product, error) {
	var snapshot productentity.Snapshot
	if err := scanner.Scan(
		&snapshot.ID,
		&snapshot.Code,
		&snapshot.Description,
		&snapshot.Balance,
		&snapshot.Version,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	); err != nil {
		return productentity.Product{}, err
	}
	return productentity.Rehydrate(snapshot)
}
