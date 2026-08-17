package product

import (
	"context"
	"errors"
	"strings"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

const (
	defaultPageSize = 50
	maximumPageSize = 100
	lowStockMaximum = int64(5)
)

// ErrInvalidStockFilter indicates an unsupported product stock filter.
var ErrInvalidStockFilter = errors.New("stock status filter must be all, active, low, or empty")

// StockFilter identifies the balance classification requested by a product query.
type StockFilter string

const (
	// StockFilterAll includes every balance.
	StockFilterAll StockFilter = "all"
	// StockFilterActive includes products above the low-stock threshold.
	StockFilterActive StockFilter = "active"
	// StockFilterLow includes products with one to five units.
	StockFilterLow StockFilter = "low"
	// StockFilterEmpty includes products without stock.
	StockFilterEmpty StockFilter = "empty"
)

// ListProductsInput contains remote filtering and pagination parameters.
type ListProductsInput struct {
	Search      string
	StockFilter StockFilter
	Limit       int
	Offset      int
}

// ListProductsOutput contains one product page and its filtered total.
type ListProductsOutput struct {
	Items  []productentity.Product
	Total  int64
	Limit  int
	Offset int
}

// ListProducts retrieves a page of products.
type ListProducts struct {
	repository productrepo.Lister
}

// NewListProducts builds the list-products use case.
func NewListProducts(repository productrepo.Lister) *ListProducts {
	return &ListProducts{repository: repository}
}

// Execute lists products using safe remote filters and pagination limits.
func (useCase *ListProducts) Execute(ctx context.Context, input ListProductsInput) (ListProductsOutput, error) {
	if input.Limit <= 0 {
		input.Limit = defaultPageSize
	}
	if input.Limit > maximumPageSize {
		input.Limit = maximumPageSize
	}
	if input.Offset < 0 {
		input.Offset = 0
	}

	criteria := productrepo.ListCriteria{
		Search: strings.TrimSpace(input.Search),
		Limit:  input.Limit,
		Offset: input.Offset,
	}
	if err := applyStockFilter(&criteria, input.StockFilter); err != nil {
		return ListProductsOutput{}, err
	}

	result, err := useCase.repository.List(ctx, criteria)
	if err != nil {
		return ListProductsOutput{}, err
	}
	return ListProductsOutput{
		Items: result.Items, Total: result.Total, Limit: input.Limit, Offset: input.Offset,
	}, nil
}

func applyStockFilter(criteria *productrepo.ListCriteria, filter StockFilter) error {
	switch filter {
	case "", StockFilterAll:
		return nil
	case StockFilterActive:
		minimum := lowStockMaximum + 1
		criteria.MinimumBalance = &minimum
	case StockFilterLow:
		minimum, maximum := int64(1), lowStockMaximum
		criteria.MinimumBalance = &minimum
		criteria.MaximumBalance = &maximum
	case StockFilterEmpty:
		empty := int64(0)
		criteria.MinimumBalance = &empty
		criteria.MaximumBalance = &empty
	default:
		return ErrInvalidStockFilter
	}
	return nil
}
