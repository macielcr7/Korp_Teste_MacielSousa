package product

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
)

type creatorStub struct {
	created productentity.Product
	err     error
}

func (stub *creatorStub) Create(_ context.Context, product productentity.Product) error {
	stub.created = product
	return stub.err
}

type idStub struct{}

func (idStub) NewID() string { return "product-id" }

type clockStub struct{ now time.Time }

func (stub clockStub) Now() time.Time { return stub.now }

func TestCreateProductExecute(t *testing.T) {
	repository := &creatorStub{}
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	useCase := NewCreateProduct(repository, idStub{}, clockStub{now: now})

	got, err := useCase.Execute(context.Background(), CreateProductInput{
		Code: "sku-1", Description: "Product", Balance: 20,
	})

	require.NoError(t, err)
	assert.Equal(t, "product-id", got.ID())
	assert.Equal(t, "SKU-1", repository.created.Code())
	assert.Equal(t, now, repository.created.CreatedAt())
}

func TestCreateProductExecuteRejectsNegativeBalance(t *testing.T) {
	useCase := NewCreateProduct(&creatorStub{}, idStub{}, clockStub{now: time.Now()})
	_, err := useCase.Execute(context.Background(), CreateProductInput{
		Code: "sku", Description: "Product", Balance: -1,
	})
	assert.ErrorIs(t, err, productentity.ErrInvalidBalance)
}
