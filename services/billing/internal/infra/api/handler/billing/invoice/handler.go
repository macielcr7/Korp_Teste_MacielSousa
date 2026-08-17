// Package invoice handles Billing invoice HTTP resources.
package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	invoiceusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/invoice"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	sharedhandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/shared"
)

type creator interface {
	Execute(context.Context, invoiceusecase.CreateInput) (invoiceentity.Invoice, bool, error)
}
type detailGetter interface {
	Execute(context.Context, string) (invoiceusecase.Detail, error)
}
type invoiceGetter interface {
	Execute(context.Context, string) (invoiceentity.Invoice, error)
}
type lister interface {
	Execute(context.Context, invoiceusecase.ListInput) (invoiceusecase.ListOutput, error)
}
type closureRequester interface {
	Execute(context.Context, string, string) (operationentity.Operation, bool, error)
}

// Handler serves invoice endpoints.
type Handler struct {
	create       creator
	get          detailGetter
	list         lister
	printable    invoiceGetter
	requestClose closureRequester
}

func New(create creator, get detailGetter, list lister, printable invoiceGetter, requestClose closureRequester) *Handler {
	return &Handler{create: create, get: get, list: list, printable: printable, requestClose: requestClose}
}

func (handler *Handler) Create(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Items []struct {
			ProductID string `json:"productId"`
			Quantity  int64  `json:"quantity"`
		} `json:"items"`
	}
	if err := decodeRequest(writer, request, &body); err != nil {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_JSON", "O corpo da requisição deve conter um único objeto JSON válido.")
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Não foi possível identificar a solicitação de criação da nota.")
		return
	}
	if len(idempotencyKey) > 200 {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "A identificação da solicitação deve conter no máximo 200 caracteres.")
		return
	}
	input := invoiceusecase.CreateInput{IdempotencyKey: idempotencyKey, Items: make([]invoiceusecase.CreateInputItem, 0, len(body.Items))}
	for _, item := range body.Items {
		productID, err := uuid.Parse(strings.TrimSpace(item.ProductID))
		if err != nil {
			sharedhandler.WriteCode(writer, request, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Selecione um produto válido.")
			return
		}
		input.Items = append(input.Items, invoiceusecase.CreateInputItem{ProductID: productID.String(), Quantity: item.Quantity})
	}
	created, replayed, err := handler.create.Execute(request.Context(), input)
	if err != nil {
		sharedhandler.WriteError(writer, request, err)
		return
	}
	if replayed {
		writer.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(writer, http.StatusCreated, mapInvoice(created))
}

func (handler *Handler) Get(writer http.ResponseWriter, request *http.Request) {
	id, ok := validID(writer, request)
	if !ok {
		return
	}
	detail, err := handler.get.Execute(request.Context(), id)
	if err != nil {
		sharedhandler.WriteError(writer, request, err)
		return
	}
	response := mapInvoice(detail.Invoice)
	if detail.ActiveClosureOperation != nil {
		response.ActiveClosureOperation = &activeClosureOperationResponse{
			OperationID: detail.ActiveClosureOperation.OperationID,
			Status:      detail.ActiveClosureOperation.Status,
		}
	}
	writeJSON(writer, http.StatusOK, response)
}

func (handler *Handler) List(writer http.ResponseWriter, request *http.Request) {
	limit, err := parseListInteger(request.URL.Query().Get("limit"), 1, 100)
	if err != nil {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_QUERY", "O limite deve ser um número inteiro entre 1 e 100.")
		return
	}
	offset, err := parseListInteger(request.URL.Query().Get("offset"), 0, 0)
	if err != nil {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_QUERY", "O deslocamento deve ser um número inteiro maior ou igual a zero.")
		return
	}
	result, err := handler.list.Execute(request.Context(), invoiceusecase.ListInput{
		Status: invoiceentity.Status(request.URL.Query().Get("status")), Limit: limit, Offset: offset,
	})
	if err != nil {
		if errors.Is(err, invoiceusecase.ErrInvalidListStatus) {
			sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_QUERY", "O status deve ser OPEN ou CLOSED.")
			return
		}
		sharedhandler.WriteError(writer, request, err)
		return
	}
	items := make([]invoiceResponse, 0, len(result.Items))
	for _, invoice := range result.Items {
		items = append(items, mapInvoice(invoice))
	}
	writeJSON(writer, http.StatusOK, struct {
		Items  []invoiceResponse `json:"items"`
		Total  int64             `json:"total"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}{Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset})
}

func parseListInteger(raw string, minimum, maximum int) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || maximum > 0 && value > maximum {
		return 0, errors.New("integer query parameter is outside the allowed range")
	}
	return value, nil
}

func (handler *Handler) RequestClosure(writer http.ResponseWriter, request *http.Request) {
	id, ok := validID(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) > 200 {
		sharedhandler.WriteCode(writer, request, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "A identificação da solicitação deve conter no máximo 200 caracteres.")
		return
	}
	operation, replayed, err := handler.requestClose.Execute(request.Context(), id, idempotencyKey)
	if err != nil {
		sharedhandler.WriteError(writer, request, err)
		return
	}
	if replayed {
		writer.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(writer, http.StatusAccepted, struct {
		OperationID string                 `json:"operationId"`
		Status      operationentity.Status `json:"status"`
	}{OperationID: operation.ID(), Status: operation.Status()})
}

func (handler *Handler) Printable(writer http.ResponseWriter, request *http.Request) {
	id, ok := validID(writer, request)
	if !ok {
		return
	}
	result, err := handler.printable.Execute(request.Context(), id)
	if err != nil {
		sharedhandler.WriteError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, mapInvoice(result))
}

type invoiceResponse struct {
	ID                     string                          `json:"id"`
	Number                 int64                           `json:"number"`
	Status                 invoiceentity.Status            `json:"status"`
	Items                  []itemResponse                  `json:"items"`
	Version                int64                           `json:"version"`
	CreatedAt              time.Time                       `json:"createdAt"`
	ClosedAt               *time.Time                      `json:"closedAt"`
	ActiveClosureOperation *activeClosureOperationResponse `json:"activeClosureOperation"`
}

type activeClosureOperationResponse struct {
	OperationID string                 `json:"operationId"`
	Status      operationentity.Status `json:"status"`
}

type itemResponse struct {
	ProductID   string `json:"productId"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
}

func mapInvoice(invoice invoiceentity.Invoice) invoiceResponse {
	items := make([]itemResponse, 0, len(invoice.Items()))
	for _, item := range invoice.Items() {
		items = append(items, itemResponse{ProductID: item.ProductID(), Code: item.Code(), Description: item.Description(), Quantity: item.Quantity()})
	}
	return invoiceResponse{ID: invoice.ID(), Number: invoice.Number(), Status: invoice.Status(), Items: items, Version: invoice.Version(), CreatedAt: invoice.CreatedAt(), ClosedAt: invoice.ClosedAt()}
}

func validID(writer http.ResponseWriter, request *http.Request) (string, bool) {
	id := chi.URLParam(request, "id")
	parsed, err := uuid.Parse(id)
	if err != nil {
		sharedhandler.WriteCode(writer, request, http.StatusUnprocessableEntity, "INVALID_ID", "O identificador do recurso deve ser um UUID válido.")
		return "", false
	}
	return parsed.String(), true
}

func decodeRequest(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
