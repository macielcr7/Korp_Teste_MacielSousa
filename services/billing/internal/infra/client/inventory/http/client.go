// Package http implements the Inventory HTTP gateway.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strings"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
)

const jsonMediaType = "application/json"

// Client calls the Inventory service using its public and internal contracts.
type Client struct {
	baseURL       string
	internalToken string
	http          *stdhttp.Client
}

func New(baseURL, internalToken string, client *stdhttp.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), internalToken: internalToken, http: client}
}

func (client *Client) FindProduct(ctx context.Context, productID string) (inventorygateway.Product, error) {
	request, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet, client.baseURL+"/api/inventory/v1/products/"+url.PathEscape(productID), nil)
	if err != nil {
		return inventorygateway.Product{}, fmt.Errorf("build inventory request: %w", err)
	}
	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("X-Internal-Token", client.internalToken)
	response, err := client.http.Do(request)
	if err != nil {
		return inventorygateway.Product{}, fmt.Errorf("%w: %v", inventorygateway.ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == stdhttp.StatusNotFound {
		code, detail := readProblem(response.Body)
		if normalizeProblemCode(code) == "PRODUCT_NOT_FOUND" {
			return inventorygateway.Product{}, inventorygateway.ErrProductNotFound
		}
		return inventorygateway.Product{}, inventoryResponseError(response.StatusCode, code, detail)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return inventorygateway.Product{}, responseError(response)
	}
	var body struct {
		ID          string `json:"id"`
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if err := decode(response.Body, &body); err != nil {
		return inventorygateway.Product{}, fmt.Errorf("%w: invalid product response: %v", inventorygateway.ErrUnavailable, err)
	}
	return inventorygateway.Product{ID: body.ID, Code: body.Code, Description: body.Description}, nil
}

func (client *Client) CommitDebit(ctx context.Context, command inventorygateway.DebitCommand) error {
	type item struct {
		ProductID string `json:"productId"`
		Quantity  int64  `json:"quantity"`
	}
	body := struct {
		CommandID string `json:"commandId"`
		InvoiceID string `json:"invoiceId"`
		Items     []item `json:"items"`
	}{CommandID: command.CommandID, InvoiceID: command.InvoiceID, Items: make([]item, 0, len(command.Items))}
	for _, commandItem := range command.Items {
		body.Items = append(body.Items, item{ProductID: commandItem.ProductID, Quantity: commandItem.Quantity})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal stock debit: %w", err)
	}
	request, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodPost, client.baseURL+"/internal/inventory/v1/stock-debits", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build stock debit request: %w", err)
	}
	request.Header.Set("Content-Type", jsonMediaType)
	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("X-Request-ID", command.CommandID)
	request.Header.Set("X-Internal-Token", client.internalToken)
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", inventorygateway.ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil
	}
	if response.StatusCode == 400 || response.StatusCode == 404 || response.StatusCode == 409 || response.StatusCode == 422 {
		code, detail := readProblem(response.Body)
		if isTerminalDebitCode(code) {
			return &inventorygateway.RejectedError{Code: code, Detail: detail}
		}
		return inventoryResponseError(response.StatusCode, code, detail)
	}
	return responseError(response)
}

func responseError(response *stdhttp.Response) error {
	code, detail := readProblem(response.Body)
	return inventoryResponseError(response.StatusCode, code, detail)
}

func inventoryResponseError(statusCode int, code, detail string) error {
	if detail == "" {
		detail = stdhttp.StatusText(statusCode)
	}
	return fmt.Errorf("%w: inventory returned %d (%s): %s", inventorygateway.ErrUnavailable, statusCode, code, detail)
}

func isTerminalDebitCode(code string) bool {
	switch normalizeProblemCode(code) {
	case "PRODUCT_NOT_FOUND", "INSUFFICIENT_STOCK", "INVALID_STOCK_COMMAND", "IDEMPOTENCY_CONFLICT":
		return true
	default:
		return false
	}
}

func normalizeProblemCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func readProblem(reader io.Reader) (string, string) {
	var problem struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := decode(reader, &problem); err != nil {
		return "INVENTORY_ERROR", ""
	}
	if problem.Code == "" {
		problem.Code = "INVENTORY_ERROR"
	}
	return problem.Code, problem.Detail
}

func decode(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("response contains multiple JSON values")
	}
	return nil
}
