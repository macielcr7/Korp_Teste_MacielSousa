// Package product contains inventory product use cases.
package product

import (
	"context"

	"github.com/macielcr7/korp-teste/services/inventory/internal/application/service/shared"
	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

// CreateProductInput contains the data required to create a product.
type CreateProductInput struct {
	Code        string
	Description string
	Balance     int64
}

// CreateProduct creates and persists a valid inventory product.
type CreateProduct struct {
	repository  productrepo.Creator
	idGenerator shared.IDGenerator
	clock       shared.Clock
}

// NewCreateProduct builds the create-product use case.
func NewCreateProduct(repository productrepo.Creator, idGenerator shared.IDGenerator, clock shared.Clock) *CreateProduct {
	return &CreateProduct{repository: repository, idGenerator: idGenerator, clock: clock}
}

// Execute creates a product.
func (useCase *CreateProduct) Execute(ctx context.Context, input CreateProductInput) (productentity.Product, error) {
	created, err := productentity.New(
		useCase.idGenerator.NewID(),
		input.Code,
		input.Description,
		input.Balance,
		useCase.clock.Now(),
	)
	if err != nil {
		return productentity.Product{}, err
	}

	if err := useCase.repository.Create(ctx, created); err != nil {
		return productentity.Product{}, err
	}
	return created, nil
}
