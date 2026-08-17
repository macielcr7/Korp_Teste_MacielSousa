// Package stockcommand exposes stock-command HTTP handlers.
package stockcommand

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	commandusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/stockcommand"
	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	"github.com/macielcr7/korp-teste/services/inventory/internal/infra/api/httpx"
)

type commitExecutor interface {
	Execute(ctx context.Context, input commandusecase.CommitStockDebitInput) (commandentity.StockCommand, error)
}

type getExecutor interface {
	Execute(ctx context.Context, commandID string) (commandentity.StockCommand, error)
}

// Handler exposes stock debit commands over HTTP.
type Handler struct {
	commit commitExecutor
	get    getExecutor
}

// New creates a stock-command handler.
func New(commit commitExecutor, get getExecutor) *Handler {
	return &Handler{commit: commit, get: get}
}

type debitRequest struct {
	CommandID string         `json:"commandId"`
	InvoiceID string         `json:"invoiceId"`
	Items     []debitItemDTO `json:"items"`
}

type debitItemDTO struct {
	ProductID string `json:"productId"`
	Quantity  int64  `json:"quantity"`
}

// Commit handles POST /internal/inventory/v1/stock-debits.
func (handler *Handler) Commit(response http.ResponseWriter, request *http.Request) {
	var body debitRequest
	if err := httpx.DecodeJSON(response, request, &body); err != nil {
		writeCommandError(response, request, errors.New("invalid JSON body"), "")
		return
	}
	commandID, err := canonicalUUID(body.CommandID)
	if err != nil {
		writeCommandError(response, request, commandentity.ErrInvalidCommand, body.CommandID)
		return
	}
	invoiceID, err := canonicalUUID(body.InvoiceID)
	if err != nil {
		writeCommandError(response, request, commandentity.ErrInvalidCommand, commandID)
		return
	}
	items := make([]commandentity.Item, 0, len(body.Items))
	for _, item := range body.Items {
		productID, parseErr := canonicalUUID(item.ProductID)
		if parseErr != nil {
			writeCommandError(response, request, commandentity.ErrInvalidCommand, commandID)
			return
		}
		items = append(items, commandentity.Item{ProductID: productID, Quantity: item.Quantity})
	}

	result, err := handler.commit.Execute(request.Context(), commandusecase.CommitStockDebitInput{
		CommandID: commandID, InvoiceID: invoiceID, Items: items,
	})
	if err != nil {
		writeCommandError(response, request, err, commandID)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, toResponse(result))
}

// Get handles GET /internal/inventory/v1/stock-debits/{commandId}.
func (handler *Handler) Get(response http.ResponseWriter, request *http.Request) {
	rawCommandID := chi.URLParam(request, "commandId")
	commandID, err := canonicalUUID(rawCommandID)
	if err != nil {
		writeCommandError(response, request, commandentity.ErrInvalidCommand, rawCommandID)
		return
	}
	result, err := handler.get.Execute(request.Context(), commandID)
	if err != nil {
		writeCommandError(response, request, err, commandID)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, toResponse(result))
}

func canonicalUUID(raw string) (string, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

type responseDTO struct {
	CommandID   string         `json:"commandId"`
	InvoiceID   string         `json:"invoiceId"`
	Status      string         `json:"status"`
	ErrorCode   string         `json:"errorCode,omitempty"`
	ErrorDetail string         `json:"errorDetail,omitempty"`
	Items       []debitItemDTO `json:"items"`
	Movements   []movementDTO  `json:"movements"`
	CreatedAt   string         `json:"createdAt"`
}

type movementDTO struct {
	ID            string `json:"id"`
	ProductID     string `json:"productId"`
	Quantity      int64  `json:"quantity"`
	BalanceBefore int64  `json:"balanceBefore"`
	BalanceAfter  int64  `json:"balanceAfter"`
	CreatedAt     string `json:"createdAt"`
}

func toResponse(command commandentity.StockCommand) responseDTO {
	items := command.Items()
	movements := command.Movements()
	response := responseDTO{
		CommandID: command.CommandID(), InvoiceID: command.InvoiceID(), Status: command.Status(),
		ErrorCode: command.ErrorCode(), ErrorDetail: command.ErrorDetail(),
		Items:     make([]debitItemDTO, 0, len(items)),
		Movements: make([]movementDTO, 0, len(movements)),
		CreatedAt: command.CreatedAt().Format(time.RFC3339Nano),
	}
	for _, item := range items {
		response.Items = append(response.Items, debitItemDTO{ProductID: item.ProductID, Quantity: item.Quantity})
	}
	for _, movement := range movements {
		response.Movements = append(response.Movements, movementDTO{
			ID: movement.ID, ProductID: movement.ProductID, Quantity: movement.Quantity,
			BalanceBefore: movement.BalanceBefore, BalanceAfter: movement.BalanceAfter,
			CreatedAt: movement.CreatedAt.Format(time.RFC3339Nano),
		})
	}
	return response
}

func writeCommandError(response http.ResponseWriter, request *http.Request, err error, commandID string) {
	problem := httpx.Problem{
		Type: "urn:korp:problem:inventory-unavailable", Title: "Serviço de estoque indisponível",
		Status: http.StatusServiceUnavailable, Code: "INVENTORY_UNAVAILABLE",
		Detail: "O estoque não conseguiu registrar a baixa. A emissão poderá ser tentada novamente automaticamente.", Retryable: true, CommandID: commandID,
	}
	switch {
	case errors.Is(err, commandentity.ErrNotFound):
		problem = httpx.Problem{Type: "urn:korp:problem:stock-command-not-found", Title: "Movimentação não encontrada", Status: http.StatusNotFound, Code: "STOCK_COMMAND_NOT_FOUND", Detail: "A movimentação de estoque não foi encontrada.", CommandID: commandID}
	case errors.Is(err, commandentity.ErrProductNotFound):
		problem = httpx.Problem{Type: "urn:korp:problem:product-not-found", Title: "Produto não encontrado", Status: http.StatusNotFound, Code: "PRODUCT_NOT_FOUND", Detail: "Um dos produtos selecionados não foi encontrado.", CommandID: commandID}
	case errors.Is(err, commandentity.ErrInsufficientStock):
		problem = httpx.Problem{Type: "urn:korp:problem:insufficient-stock", Title: "Saldo insuficiente", Status: http.StatusConflict, Code: "INSUFFICIENT_STOCK", Detail: "Saldo insuficiente para um ou mais produtos da nota.", CommandID: commandID}
	case errors.Is(err, commandentity.ErrIdempotencyConflict):
		problem = httpx.Problem{Type: "urn:korp:problem:idempotency-conflict", Title: "Conflito na solicitação", Status: http.StatusConflict, Code: "IDEMPOTENCY_CONFLICT", Detail: "A solicitação já foi usada com dados diferentes.", CommandID: commandID}
	case errors.Is(err, commandentity.ErrInvalidCommand), errors.Is(err, commandentity.ErrDuplicateProduct), errors.Is(err, commandentity.ErrInvalidQuantity):
		problem = httpx.Problem{Type: "urn:korp:problem:invalid-stock-command", Title: "Movimentação inválida", Status: http.StatusUnprocessableEntity, Code: "INVALID_STOCK_COMMAND", Detail: "Revise os produtos e as quantidades informadas.", CommandID: commandID}
	case err.Error() == "invalid JSON body":
		problem = httpx.Problem{Type: "urn:korp:problem:invalid-request", Title: "Requisição inválida", Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Detail: "O corpo da requisição deve conter um objeto JSON válido.", CommandID: commandID}
	}
	httpx.WriteProblem(response, request, problem)
}
