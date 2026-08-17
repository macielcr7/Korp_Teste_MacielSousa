// Package shared contains Billing HTTP response helpers.
package shared

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
	closureusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/closureoperation"
	invoiceusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/invoice"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/api/middleware"
)

// Problem is an RFC 9457-compatible API error with stable application fields.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail"`
	TraceID   string       `json:"traceId"`
	Retryable bool         `json:"retryable"`
	Errors    []FieldError `json:"errors"`
}

// FieldError identifies a request-field validation issue.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// WriteError maps application and domain errors to safe Problem Details.
func WriteError(writer http.ResponseWriter, request *http.Request, err error) {
	problem := classify(err)
	problem.TraceID = middleware.RequestID(request.Context())
	if problem.Status >= 500 {
		slog.ErrorContext(request.Context(), "request failed", "request_id", problem.TraceID, "error", err)
	}
	Write(writer, problem)
}

// WriteCode writes an explicitly classified client error.
func WriteCode(writer http.ResponseWriter, request *http.Request, status int, code, detail string) {
	Write(writer, Problem{Type: "https://korp.dev/problems/" + code, Title: statusTitle(status), Status: status, Code: code, Detail: detail, TraceID: middleware.RequestID(request.Context()), Errors: []FieldError{}})
}

// Write serializes Problem Details.
func Write(writer http.ResponseWriter, problem Problem) {
	if problem.Errors == nil {
		problem.Errors = []FieldError{}
	}
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(problem.Status)
	_ = json.NewEncoder(writer).Encode(problem)
}

func classify(err error) Problem {
	switch {
	case errors.Is(err, invoicerepository.ErrNotFound):
		return makeProblem(http.StatusNotFound, "INVOICE_NOT_FOUND", "A nota não foi encontrada.", false)
	case errors.Is(err, operationrepository.ErrNotFound):
		return makeProblem(http.StatusNotFound, "CLOSURE_OPERATION_NOT_FOUND", "A operação de emissão não foi encontrada.", false)
	case errors.Is(err, inventorygateway.ErrProductNotFound):
		return makeProblem(http.StatusNotFound, "PRODUCT_NOT_FOUND", "Um dos produtos selecionados não foi encontrado.", false)
	case errors.Is(err, inventorygateway.ErrUnavailable):
		return makeProblem(http.StatusServiceUnavailable, "INVENTORY_UNAVAILABLE", "O serviço de estoque está temporariamente indisponível.", true)
	case errors.Is(err, closureusecase.ErrIdempotencyKeyRequired):
		return makeProblem(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Não foi possível identificar a solicitação de emissão.", false)
	case errors.Is(err, operationrepository.ErrIdempotencyKeyConflict):
		return makeProblem(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "A chave de emissão já foi usada para outra nota.", false)
	case errors.Is(err, invoicerepository.ErrIdempotencyKeyConflict):
		return makeProblem(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "A identificação da solicitação já foi usada com dados diferentes.", false)
	case errors.Is(err, invoiceusecase.ErrIdempotencyKeyRequired):
		return makeProblem(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Não foi possível identificar a solicitação de criação da nota.", false)
	case errors.Is(err, invoiceusecase.ErrIdempotencyKeyTooLong):
		return makeProblem(http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "A identificação da solicitação deve conter no máximo 200 caracteres.", false)
	case errors.Is(err, invoiceusecase.ErrTooManyItems):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A nota deve conter no máximo 20 itens.", false)
	case errors.Is(err, operationrepository.ErrActiveOperationExists):
		return makeProblem(http.StatusConflict, "CLOSURE_ALREADY_REQUESTED", "A emissão desta nota já foi solicitada.", false)
	case errors.Is(err, invoiceentity.ErrInvoiceNotOpen):
		return makeProblem(http.StatusConflict, "INVOICE_NOT_OPEN", "A nota não está mais aberta.", false)
	case errors.Is(err, invoiceentity.ErrInvoiceNotClosed):
		return makeProblem(http.StatusConflict, "INVOICE_NOT_CLOSED", "Somente notas emitidas podem ser impressas.", false)
	case errors.Is(err, invoiceentity.ErrItemsRequired):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Informe pelo menos um item para a nota.", false)
	case errors.Is(err, invoiceentity.ErrInvalidProductID):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Selecione um produto válido.", false)
	case errors.Is(err, invoiceentity.ErrInvalidProductCode):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "O código do produto é obrigatório.", false)
	case errors.Is(err, invoiceentity.ErrInvalidDescription):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A descrição do produto é obrigatória.", false)
	case errors.Is(err, invoiceentity.ErrInvalidQuantity):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A quantidade deve ser um número inteiro válido maior que zero.", false)
	case errors.Is(err, invoiceentity.ErrDuplicateProduct):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Cada produto pode aparecer somente uma vez na nota.", false)
	case errors.Is(err, operationentity.ErrInvalidOperation):
		return makeProblem(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A operação de emissão é inválida.", false)
	default:
		return makeProblem(http.StatusInternalServerError, "INTERNAL_ERROR", "O servidor não conseguiu processar a solicitação. Tente novamente e, se o problema persistir, informe o código de rastreio.", true)
	}
}

func makeProblem(status int, code, detail string, retryable bool) Problem {
	return Problem{Type: "https://korp.dev/problems/" + code, Title: statusTitle(status), Status: status, Code: code, Detail: detail, Retryable: retryable, Errors: []FieldError{}}
}

// LocalizeOperationError converts legacy technical messages before they cross the HTTP boundary.
func LocalizeOperationError(detail string) string {
	normalized := strings.ToLower(strings.TrimSpace(detail))
	switch {
	case strings.Contains(normalized, "insufficient product balance"), strings.Contains(normalized, "insufficient stock"), strings.Contains(normalized, "saldo insuficiente"):
		return "Saldo insuficiente para um ou mais produtos da nota."
	case strings.Contains(normalized, "invoice no longer exists"), strings.Contains(normalized, "a nota não foi encontrada"):
		return "A nota não foi encontrada."
	case strings.Contains(normalized, "product not found"), strings.Contains(normalized, "produto da nota não foi encontrado"), strings.Contains(normalized, "produtos da nota não foi encontrado"):
		return "Um dos produtos da nota não foi encontrado."
	case strings.Contains(normalized, "invalid stock command"), strings.Contains(normalized, "dados da baixa de estoque são inválidos"):
		return "Os dados da baixa de estoque são inválidos. Revise os produtos e as quantidades da nota."
	case strings.Contains(normalized, "idempotency conflict"), strings.Contains(normalized, "baixa de estoque desta nota entrou em conflito"):
		return "A baixa de estoque desta nota entrou em conflito com uma solicitação anterior."
	case strings.Contains(normalized, "serviço de estoque não respondeu"), strings.Contains(normalized, "inventory unavailable"), strings.Contains(normalized, "connection refused"):
		return "O serviço de estoque não respondeu. A emissão será tentada novamente automaticamente."
	default:
		if normalized == "" {
			return ""
		}
		return "A emissão foi rejeitada pelo estoque. Verifique os produtos e as quantidades da nota antes de tentar novamente."
	}
}

func statusTitle(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Requisição inválida"
	case http.StatusNotFound:
		return "Recurso não encontrado"
	case http.StatusConflict:
		return "Conflito"
	case http.StatusUnprocessableEntity:
		return "Dados inválidos"
	case http.StatusServiceUnavailable:
		return "Serviço indisponível"
	default:
		return "Erro interno"
	}
}
