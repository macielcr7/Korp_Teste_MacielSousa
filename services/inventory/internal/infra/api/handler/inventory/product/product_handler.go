// Package product exposes inventory product HTTP handlers.
package product

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	productusecase "github.com/macielcr7/korp-teste/services/inventory/internal/application/usecase/inventory/product"
	productentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/product"
	"github.com/macielcr7/korp-teste/services/inventory/internal/infra/api/httpx"
)

type createExecutor interface {
	Execute(ctx context.Context, input productusecase.CreateProductInput) (productentity.Product, error)
}

type getExecutor interface {
	Execute(ctx context.Context, id string) (productentity.Product, error)
}

type listExecutor interface {
	Execute(ctx context.Context, input productusecase.ListProductsInput) (productusecase.ListProductsOutput, error)
}

// Handler exposes product use cases over HTTP.
type Handler struct {
	create createExecutor
	get    getExecutor
	list   listExecutor
}

// New creates a product handler.
func New(create createExecutor, get getExecutor, list listExecutor) *Handler {
	return &Handler{create: create, get: get, list: list}
}

type createRequest struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Balance     int64  `json:"balance"`
}

var (
	errInvalidLimit  = errors.New("limit must be an integer between 1 and 100")
	errInvalidOffset = errors.New("offset must be a non-negative integer")
)

const invalidProductTitle = "Produto inválido"

// Create handles POST /api/inventory/v1/products.
func (handler *Handler) Create(response http.ResponseWriter, request *http.Request) {
	var body createRequest
	if err := httpx.DecodeJSON(response, request, &body); err != nil {
		writeProductError(response, request, errors.New("invalid JSON body"))
		return
	}

	created, err := handler.create.Execute(request.Context(), productusecase.CreateProductInput{
		Code: body.Code, Description: body.Description, Balance: body.Balance,
	})
	if err != nil {
		writeProductError(response, request, err)
		return
	}

	response.Header().Set("Location", "/api/inventory/v1/products/"+created.ID())
	httpx.WriteJSON(response, http.StatusCreated, toResponse(created))
}

// Get handles GET /api/inventory/v1/products/{id}.
func (handler *Handler) Get(response http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")
	parsed, err := uuid.Parse(id)
	if err != nil {
		writeProductError(response, request, productentity.ErrInvalidID)
		return
	}
	found, err := handler.get.Execute(request.Context(), parsed.String())
	if err != nil {
		writeProductError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, toResponse(found))
}

// List handles GET /api/inventory/v1/products.
func (handler *Handler) List(response http.ResponseWriter, request *http.Request) {
	limit, err := parseInteger(request.URL.Query().Get("limit"), 1, 100)
	if err != nil {
		writeProductError(response, request, errInvalidLimit)
		return
	}
	offset, err := parseInteger(request.URL.Query().Get("offset"), 0, 0)
	if err != nil {
		writeProductError(response, request, errInvalidOffset)
		return
	}

	query := request.URL.Query()
	result, err := handler.list.Execute(request.Context(), productusecase.ListProductsInput{
		Search: query.Get("search"), StockFilter: productusecase.StockFilter(query.Get("status")),
		Limit: limit, Offset: offset,
	})
	if err != nil {
		writeProductError(response, request, err)
		return
	}
	items := make([]responseDTO, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toResponse(item))
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{
		"items": items, "total": result.Total, "limit": result.Limit, "offset": result.Offset,
	})
}

type responseDTO struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Balance     int64  `json:"balance"`
	Version     int64  `json:"version"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func toResponse(product productentity.Product) responseDTO {
	return responseDTO{
		ID: product.ID(), Code: product.Code(), Description: product.Description(),
		Balance: product.Balance(), Version: product.Version(),
		CreatedAt: product.CreatedAt().Format(time.RFC3339Nano),
		UpdatedAt: product.UpdatedAt().Format(time.RFC3339Nano),
	}
}

func parseInteger(raw string, minimum, maximum int) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || maximum > 0 && value > maximum {
		return 0, errors.New("integer query parameter is outside the allowed range")
	}
	return value, nil
}

func writeProductError(response http.ResponseWriter, request *http.Request, err error) {
	problem := httpx.Problem{
		Type: "urn:korp:problem:internal-error", Title: "Erro interno",
		Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR",
		Detail: "O catálogo de produtos não conseguiu processar a operação. Tente novamente e, se o problema persistir, informe o código de rastreio.", Retryable: true,
	}
	switch {
	case errors.Is(err, productentity.ErrNotFound):
		problem = httpx.Problem{Type: "urn:korp:problem:product-not-found", Title: "Produto não encontrado", Status: http.StatusNotFound, Code: "PRODUCT_NOT_FOUND", Detail: "O produto não foi encontrado."}
	case errors.Is(err, productentity.ErrDuplicateCode):
		problem = productValidationProblem("code", http.StatusConflict, "DUPLICATE_PRODUCT_CODE", "Código já cadastrado", "Já existe um produto com este código.")
	case errors.Is(err, productentity.ErrInvalidID):
		problem = productValidationProblem("id", http.StatusUnprocessableEntity, "INVALID_PRODUCT", invalidProductTitle, "O identificador do produto é obrigatório.")
	case errors.Is(err, productentity.ErrInvalidCode):
		problem = productValidationProblem("code", http.StatusUnprocessableEntity, "INVALID_PRODUCT", invalidProductTitle, "O código deve conter de 1 a 64 letras, números, pontos, sublinhados ou hífens.")
	case errors.Is(err, productentity.ErrInvalidDescription):
		problem = productValidationProblem("description", http.StatusUnprocessableEntity, "INVALID_PRODUCT", invalidProductTitle, "A descrição deve conter entre 1 e 255 caracteres.")
	case errors.Is(err, productentity.ErrInvalidBalance):
		problem = productValidationProblem("balance", http.StatusUnprocessableEntity, "INVALID_PRODUCT", invalidProductTitle, "O saldo deve ser um número inteiro válido maior ou igual a zero.")
	case errors.Is(err, errInvalidLimit):
		problem = requestFieldProblem("limit", "O limite deve ser um número inteiro entre 1 e 100.")
	case errors.Is(err, errInvalidOffset):
		problem = requestFieldProblem("offset", "O deslocamento deve ser um número inteiro maior ou igual a zero.")
	case errors.Is(err, productusecase.ErrInvalidStockFilter):
		problem = requestFieldProblem("status", "O filtro de estoque informado é inválido.")
	case err.Error() == "invalid JSON body":
		problem = httpx.Problem{Type: "urn:korp:problem:invalid-request", Title: "Requisição inválida", Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Detail: "O corpo da requisição deve conter um objeto JSON válido."}
	}
	httpx.WriteProblem(response, request, problem)
}

func productValidationProblem(field string, status int, code, title, detail string) httpx.Problem {
	return httpx.Problem{
		Type: "urn:korp:problem:invalid-product", Title: title, Status: status, Code: code, Detail: detail,
		Errors: []httpx.FieldError{{Field: field, Message: detail}},
	}
}

func requestFieldProblem(field, detail string) httpx.Problem {
	return httpx.Problem{
		Type: "urn:korp:problem:invalid-request", Title: "Requisição inválida", Status: http.StatusBadRequest,
		Code: "INVALID_REQUEST", Detail: detail, Errors: []httpx.FieldError{{Field: field, Message: detail}},
	}
}
