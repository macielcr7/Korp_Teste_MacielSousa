package product

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	productusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/product"
	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
)

type createStub struct {
	input  productusecase.CreateProductInput
	result productentity.Product
	err    error
}

func (stub *createStub) Execute(_ context.Context, input productusecase.CreateProductInput) (productentity.Product, error) {
	stub.input = input
	return stub.result, stub.err
}

type getStub struct{}

func (getStub) Execute(context.Context, string) (productentity.Product, error) {
	return productentity.Product{}, productentity.ErrNotFound
}

type capturingGetStub struct {
	id string
}

func (stub *capturingGetStub) Execute(_ context.Context, id string) (productentity.Product, error) {
	stub.id = id
	return productentity.Product{}, productentity.ErrNotFound
}

type listStub struct {
	input productusecase.ListProductsInput
}

func (stub *listStub) Execute(_ context.Context, input productusecase.ListProductsInput) (productusecase.ListProductsOutput, error) {
	stub.input = input
	return productusecase.ListProductsOutput{Items: []productentity.Product{}, Total: 17, Limit: input.Limit, Offset: input.Offset}, nil
}

func TestCreate(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	created, err := productentity.Rehydrate(productentity.Snapshot{
		ID: "9c637a57-68d2-4dcf-955d-18ce087fd574", Code: "SKU-1", Description: "Product",
		Balance: 10, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	creator := &createStub{result: created}
	handler := New(creator, getStub{}, &listStub{})
	request := httptest.NewRequest(http.MethodPost, "/api/inventory/v1/products",
		strings.NewReader(`{"code":"sku-1","description":"Product","balance":10}`))
	response := httptest.NewRecorder()

	handler.Create(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "sku-1", creator.input.Code)
	assert.Equal(t, "/api/inventory/v1/products/9c637a57-68d2-4dcf-955d-18ce087fd574", response.Header().Get("Location"))
	assert.Contains(t, response.Body.String(), `"code":"SKU-1"`)
}

func TestCreateRejectsUnknownJSONField(t *testing.T) {
	handler := New(&createStub{}, getStub{}, &listStub{})
	request := httptest.NewRequest(http.MethodPost, "/api/inventory/v1/products",
		strings.NewReader(`{"code":"sku","description":"Product","balance":1,"unknown":true}`))
	response := httptest.NewRecorder()

	handler.Create(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
	assert.Contains(t, response.Body.String(), `"code":"INVALID_REQUEST"`)
	assert.Contains(t, response.Body.String(), `"errors":[]`)
}

func TestCreateAssociatesInvalidBalanceWithField(t *testing.T) {
	creator := &createStub{err: productentity.ErrInvalidBalance}
	handler := New(creator, getStub{}, &listStub{})
	request := httptest.NewRequest(http.MethodPost, "/api/inventory/v1/products",
		strings.NewReader(`{"code":"sku","description":"Product","balance":-1}`))
	response := httptest.NewRecorder()

	handler.Create(response, request)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Contains(t, response.Body.String(), `"errors":[{"field":"balance","message":"O saldo deve ser um número inteiro válido maior ou igual a zero."}]`)
}

func TestGetCanonicalizesProductUUID(t *testing.T) {
	getter := &capturingGetStub{}
	handler := New(&createStub{}, getter, &listStub{})
	router := chi.NewRouter()
	router.Get("/api/inventory/v1/products/{id}", handler.Get)
	request := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products/urn:uuid:9c637a57-68d2-4dcf-955d-18ce087fd574", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "9c637a57-68d2-4dcf-955d-18ce087fd574", getter.id)
}

func TestListPassesRemoteFiltersAndPagination(t *testing.T) {
	lister := &listStub{}
	handler := New(&createStub{}, getStub{}, lister)
	request := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products?search=teclado&status=low&limit=10&offset=20", nil)
	response := httptest.NewRecorder()

	handler.List(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "teclado", lister.input.Search)
	assert.Equal(t, productusecase.StockFilterLow, lister.input.StockFilter)
	assert.Equal(t, 10, lister.input.Limit)
	assert.Equal(t, 20, lister.input.Offset)
	assert.Contains(t, response.Body.String(), `"total":17`)
}

func TestListRejectsInvalidStockFilter(t *testing.T) {
	handler := New(&createStub{}, getStub{}, NewListErrorStub(productusecase.ErrInvalidStockFilter))
	request := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products?status=unknown", nil)
	response := httptest.NewRecorder()

	handler.List(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), `"field":"status"`)
}

func TestListRejectsPaginationOutsideOpenAPIContract(t *testing.T) {
	tests := []struct {
		query string
		field string
	}{
		{query: "limit=0", field: "limit"},
		{query: "limit=101", field: "limit"},
		{query: "limit=invalid", field: "limit"},
		{query: "offset=-1", field: "offset"},
		{query: "offset=invalid", field: "offset"},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			handler := New(&createStub{}, getStub{}, &listStub{})
			request := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products?"+test.query, nil)
			response := httptest.NewRecorder()

			handler.List(response, request)

			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Contains(t, response.Body.String(), `"field":"`+test.field+`"`)
		})
	}
}

type listErrorStub struct {
	err error
}

// NewListErrorStub creates a failing list executor for handler tests.
func NewListErrorStub(err error) *listErrorStub {
	return &listErrorStub{err: err}
}

func (stub *listErrorStub) Execute(context.Context, productusecase.ListProductsInput) (productusecase.ListProductsOutput, error) {
	return productusecase.ListProductsOutput{}, stub.err
}
