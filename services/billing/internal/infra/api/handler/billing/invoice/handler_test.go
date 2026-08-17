package invoice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	closureusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/closureoperation"
	invoiceusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/invoice"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/api/middleware"
)

type closureRequesterFake struct {
	operation operationentity.Operation
	replayed  bool
	invoiceID string
	key       string
	err       error
}

type invoiceCreatorFake struct {
	input    invoiceusecase.CreateInput
	invoice  invoiceentity.Invoice
	replayed bool
	err      error
}

func (fake *invoiceCreatorFake) Execute(_ context.Context, input invoiceusecase.CreateInput) (invoiceentity.Invoice, bool, error) {
	fake.input = input
	return fake.invoice, fake.replayed, fake.err
}

func (fake *closureRequesterFake) Execute(_ context.Context, invoiceID, key string) (operationentity.Operation, bool, error) {
	fake.invoiceID = invoiceID
	fake.key = key
	return fake.operation, fake.replayed, fake.err
}

type invoiceGetterFake struct{}

func (invoiceGetterFake) Execute(context.Context, string) (invoiceentity.Invoice, error) {
	panic("not used")
}

type detailGetterFake struct {
	detail invoiceusecase.Detail
	err    error
}

type invoiceListerFake struct {
	input  invoiceusecase.ListInput
	output invoiceusecase.ListOutput
	err    error
}

func (fake *invoiceListerFake) Execute(_ context.Context, input invoiceusecase.ListInput) (invoiceusecase.ListOutput, error) {
	fake.input = input
	return fake.output, fake.err
}

func (fake detailGetterFake) Execute(context.Context, string) (invoiceusecase.Detail, error) {
	return fake.detail, fake.err
}

func TestRequestClosureReturnsAcceptedAndReplayHeader(t *testing.T) {
	now := time.Now().UTC()
	operation, err := operationentity.New("47c05c52-c49b-4a51-bfc7-13546bf745b9", "5a8fd34a-b692-4a7a-9e03-91e0beea42c9", "request-1", "97153cb3-e5e5-4f7b-86ce-71f61530362a", now)
	require.NoError(t, err)
	requester := &closureRequesterFake{operation: operation, replayed: true}
	handler := New(nil, detailGetterFake{}, nil, invoiceGetterFake{}, requester)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Post("/invoices/{id}/close", handler.RequestClosure)
	request := httptest.NewRequest(http.MethodPost, "/invoices/5a8fd34a-b692-4a7a-9e03-91e0beea42c9/close", nil)
	request.Header.Set("Idempotency-Key", "request-1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, "true", recorder.Header().Get("Idempotent-Replayed"))
	require.Equal(t, "request-1", requester.key)
	require.JSONEq(t, `{"operationId":"47c05c52-c49b-4a51-bfc7-13546bf745b9","status":"PENDING"}`, recorder.Body.String())
}

func TestCreateRequiresIdempotencyKeyAndCanonicalizesProductUUID(t *testing.T) {
	creator := &invoiceCreatorFake{}
	handler := New(creator, detailGetterFake{}, nil, invoiceGetterFake{}, nil)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Post("/invoices", handler.Create)

	missingKey := httptest.NewRecorder()
	router.ServeHTTP(missingKey, httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"items":[]}`)))
	require.Equal(t, http.StatusBadRequest, missingKey.Code)
	require.Contains(t, missingKey.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")

	invalidProduct := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"items":[{"productId":"not-a-uuid","quantity":1}]}`))
	invalidProduct.Header.Set("Idempotency-Key", "create-1")
	invalidResponse := httptest.NewRecorder()
	router.ServeHTTP(invalidResponse, invalidProduct)
	require.Equal(t, http.StatusUnprocessableEntity, invalidResponse.Code)

	validProduct := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"items":[{"productId":"B329CAD19DC0453699AF83D7BD8F7A29","quantity":1}]}`))
	validProduct.Header.Set("Idempotency-Key", "create-2")
	validResponse := httptest.NewRecorder()
	router.ServeHTTP(validResponse, validProduct)
	require.Equal(t, http.StatusCreated, validResponse.Code)
	require.Equal(t, "b329cad1-9dc0-4536-99af-83d7bd8f7a29", creator.input.Items[0].ProductID)
	require.Equal(t, "create-2", creator.input.IdempotencyKey)
}

func TestCreateSetsReplayHeader(t *testing.T) {
	creator := &invoiceCreatorFake{replayed: true}
	handler := New(creator, detailGetterFake{}, nil, invoiceGetterFake{}, nil)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Post("/invoices", handler.Create)
	request := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"items":[{"productId":"b329cad1-9dc0-4536-99af-83d7bd8f7a29","quantity":1}]}`))
	request.Header.Set("Idempotency-Key", "create-1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "true", recorder.Header().Get("Idempotent-Replayed"))
}

func TestRequestClosureRequiresIdempotencyKey(t *testing.T) {
	requester := &closureRequesterFake{err: closureusecase.ErrIdempotencyKeyRequired}
	handler := New(nil, detailGetterFake{}, nil, invoiceGetterFake{}, requester)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Post("/invoices/{id}/close", handler.RequestClosure)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/invoices/5a8fd34a-b692-4a7a-9e03-91e0beea42c9/close", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
}

func TestRequestClosureCanonicalizesInvoiceUUID(t *testing.T) {
	requester := &closureRequesterFake{}
	handler := New(nil, detailGetterFake{}, nil, invoiceGetterFake{}, requester)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Post("/invoices/{id}/close", handler.RequestClosure)
	request := httptest.NewRequest(http.MethodPost, "/invoices/urn:uuid:5a8fd34a-b692-4a7a-9e03-91e0beea42c9/close", nil)
	request.Header.Set("Idempotency-Key", "request-1")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, "5a8fd34a-b692-4a7a-9e03-91e0beea42c9", requester.invoiceID)
}

func TestGetIncludesActiveClosureOperationForPollingResume(t *testing.T) {
	now := time.Now().UTC()
	item, err := invoiceentity.NewItem("product-1", "SKU-1", "Keyboard", 1)
	require.NoError(t, err)
	inv, err := invoiceentity.Rehydrate("5a8fd34a-b692-4a7a-9e03-91e0beea42c9", 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, now, nil)
	require.NoError(t, err)
	detail := invoiceusecase.Detail{
		Invoice: inv,
		ActiveClosureOperation: &invoiceusecase.ActiveClosureOperation{
			OperationID: "47c05c52-c49b-4a51-bfc7-13546bf745b9",
			Status:      operationentity.StatusRetrying,
		},
	}
	handler := New(nil, detailGetterFake{detail: detail}, nil, invoiceGetterFake{}, nil)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Get("/invoices/{id}", handler.Get)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/invoices/5a8fd34a-b692-4a7a-9e03-91e0beea42c9", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"activeClosureOperation":{"operationId":"47c05c52-c49b-4a51-bfc7-13546bf745b9","status":"RETRYING"}`)
}

func TestListPassesRemoteStatusAndPagination(t *testing.T) {
	lister := &invoiceListerFake{output: invoiceusecase.ListOutput{Items: []invoiceentity.Invoice{}, Total: 24, Limit: 10, Offset: 10}}
	handler := New(nil, detailGetterFake{}, lister, invoiceGetterFake{}, nil)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Get("/invoices", handler.List)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/invoices?status=CLOSED&limit=10&offset=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, invoiceentity.StatusClosed, lister.input.Status)
	require.Equal(t, 10, lister.input.Limit)
	require.Equal(t, 10, lister.input.Offset)
	require.JSONEq(t, `{"items":[],"total":24,"limit":10,"offset":10}`, recorder.Body.String())
}

func TestListRejectsInvalidStatus(t *testing.T) {
	lister := &invoiceListerFake{err: invoiceusecase.ErrInvalidListStatus}
	handler := New(nil, detailGetterFake{}, lister, invoiceGetterFake{}, nil)
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID)
	router.Get("/invoices", handler.List)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/invoices?status=INVALID", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "INVALID_QUERY")
}

func TestListRejectsPaginationOutsideContract(t *testing.T) {
	testCases := []string{
		"/invoices?limit=0",
		"/invoices?limit=101",
		"/invoices?offset=-1",
	}
	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			handler := New(nil, detailGetterFake{}, &invoiceListerFake{}, invoiceGetterFake{}, nil)
			router := chi.NewRouter()
			router.Use(middleware.WithRequestID)
			router.Get("/invoices", handler.List)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Contains(t, recorder.Body.String(), "INVALID_QUERY")
		})
	}
}
