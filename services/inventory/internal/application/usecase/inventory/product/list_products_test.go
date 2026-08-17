package product

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	productrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/product"
)

type listRepositoryStub struct {
	criteria productrepo.ListCriteria
	result   productrepo.ListResult
}

func (stub *listRepositoryStub) List(_ context.Context, criteria productrepo.ListCriteria) (productrepo.ListResult, error) {
	stub.criteria = criteria
	return stub.result, nil
}

func TestListProductsAppliesSearchStockFilterAndPagination(t *testing.T) {
	repository := &listRepositoryStub{result: productrepo.ListResult{Total: 23}}
	useCase := NewListProducts(repository)

	output, err := useCase.Execute(context.Background(), ListProductsInput{
		Search: "  teclado  ", StockFilter: StockFilterLow, Limit: 10, Offset: 20,
	})

	require.NoError(t, err)
	assert.Equal(t, "teclado", repository.criteria.Search)
	assert.Equal(t, int64(1), *repository.criteria.MinimumBalance)
	assert.Equal(t, int64(5), *repository.criteria.MaximumBalance)
	assert.Equal(t, 10, repository.criteria.Limit)
	assert.Equal(t, 20, repository.criteria.Offset)
	assert.Equal(t, int64(23), output.Total)
}

func TestListProductsMapsExclusiveStockStatuses(t *testing.T) {
	tests := []struct {
		name    string
		filter  StockFilter
		minimum *int64
		maximum *int64
	}{
		{name: "all", filter: StockFilterAll},
		{name: "active", filter: StockFilterActive, minimum: int64Pointer(6)},
		{name: "low", filter: StockFilterLow, minimum: int64Pointer(1), maximum: int64Pointer(5)},
		{name: "empty", filter: StockFilterEmpty, minimum: int64Pointer(0), maximum: int64Pointer(0)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &listRepositoryStub{}
			_, err := NewListProducts(repository).Execute(context.Background(), ListProductsInput{StockFilter: test.filter})

			require.NoError(t, err)
			assert.Equal(t, test.minimum, repository.criteria.MinimumBalance)
			assert.Equal(t, test.maximum, repository.criteria.MaximumBalance)
		})
	}
}

func TestListProductsRejectsUnknownStockFilter(t *testing.T) {
	_, err := NewListProducts(&listRepositoryStub{}).Execute(context.Background(), ListProductsInput{StockFilter: "unknown"})

	assert.ErrorIs(t, err, ErrInvalidStockFilter)
}

func int64Pointer(value int64) *int64 {
	return &value
}
