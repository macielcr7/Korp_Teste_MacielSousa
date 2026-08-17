package product

import (
	"context"
	"strings"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

// GetProduct retrieves a product by ID.
type GetProduct struct {
	repository productrepo.Finder
}

// NewGetProduct builds the get-product use case.
func NewGetProduct(repository productrepo.Finder) *GetProduct {
	return &GetProduct{repository: repository}
}

// Execute retrieves one product.
func (useCase *GetProduct) Execute(ctx context.Context, id string) (productentity.Product, error) {
	if strings.TrimSpace(id) == "" {
		return productentity.Product{}, productentity.ErrInvalidID
	}
	return useCase.repository.GetByID(ctx, id)
}
