// Package product defines persistence ports for inventory products.
package product

import (
	"context"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
)

// Creator persists a new product.
type Creator interface {
	Create(ctx context.Context, product productentity.Product) error
}

// Finder retrieves a product by ID.
type Finder interface {
	GetByID(ctx context.Context, id string) (productentity.Product, error)
}

// ListCriteria contains persistence-neutral product search constraints.
type ListCriteria struct {
	Search         string
	MinimumBalance *int64
	MaximumBalance *int64
	Limit          int
	Offset         int
}

// ListResult contains a product page and the total matching rows.
type ListResult struct {
	Items []productentity.Product
	Total int64
}

// Lister retrieves a stable page of products.
type Lister interface {
	List(ctx context.Context, criteria ListCriteria) (ListResult, error)
}
