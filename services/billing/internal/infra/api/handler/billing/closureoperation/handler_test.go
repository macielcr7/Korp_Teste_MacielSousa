package closureoperation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
)

type getterFake struct {
	id        string
	operation operationentity.Operation
}

func (fake *getterFake) Execute(_ context.Context, id string) (operationentity.Operation, error) {
	fake.id = id
	return fake.operation, nil
}

func TestGetCanonicalizesOperationUUID(t *testing.T) {
	now := time.Now().UTC()
	operation, err := operationentity.New(
		"47c05c52-c49b-4a51-bfc7-13546bf745b9",
		"5a8fd34a-b692-4a7a-9e03-91e0beea42c9",
		"request-1",
		"97153cb3-e5e5-4f7b-86ce-71f61530362a",
		now,
	)
	require.NoError(t, err)
	getter := &getterFake{operation: operation}
	handler := New(getter)
	router := chi.NewRouter()
	router.Get("/closure-operations/{id}", handler.Get)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/closure-operations/urn:uuid:47c05c52-c49b-4a51-bfc7-13546bf745b9", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "47c05c52-c49b-4a51-bfc7-13546bf745b9", getter.id)
}
