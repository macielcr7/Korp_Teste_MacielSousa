// Package closureoperation handles closure-operation HTTP resources.
package closureoperation

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	sharedhandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/shared"
)

type getter interface {
	Execute(context.Context, string) (operationentity.Operation, error)
}

// Handler serves closure-operation endpoints.
type Handler struct{ get getter }

func New(get getter) *Handler { return &Handler{get: get} }

func (handler *Handler) Get(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")
	parsed, err := uuid.Parse(id)
	if err != nil {
		sharedhandler.WriteCode(writer, request, http.StatusUnprocessableEntity, "INVALID_ID", "O identificador do recurso deve ser um UUID válido.")
		return
	}
	operation, err := handler.get.Execute(request.Context(), parsed.String())
	if err != nil {
		sharedhandler.WriteError(writer, request, err)
		return
	}
	response := struct {
		ID            string                 `json:"id"`
		InvoiceID     string                 `json:"invoiceId"`
		Status        operationentity.Status `json:"status"`
		Attempts      int                    `json:"attempts"`
		NextAttemptAt time.Time              `json:"nextAttemptAt"`
		LastError     string                 `json:"lastError,omitempty"`
		Retryable     bool                   `json:"retryable"`
		CreatedAt     time.Time              `json:"createdAt"`
		UpdatedAt     time.Time              `json:"updatedAt"`
	}{
		ID: operation.ID(), InvoiceID: operation.InvoiceID(), Status: operation.Status(), Attempts: operation.Attempts(),
		NextAttemptAt: operation.NextAttemptAt(), LastError: sharedhandler.LocalizeOperationError(operation.LastError()), Retryable: operation.Status() == operationentity.StatusRetrying,
		CreatedAt: operation.CreatedAt(), UpdatedAt: operation.UpdatedAt(),
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}
