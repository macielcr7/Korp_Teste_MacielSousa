package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/billing/inventorygateway"
)

type roundTripFunc func(*stdhttp.Request) (*stdhttp.Response, error)

func (function roundTripFunc) RoundTrip(request *stdhttp.Request) (*stdhttp.Response, error) {
	return function(request)
}

func TestCommitDebitUsesCamelCaseContract(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		require.Equal(t, "/internal/inventory/v1/stock-debits", request.URL.Path)
		require.Equal(t, "application/json", request.Header.Get("Content-Type"))
		require.Equal(t, "command-1", request.Header.Get("X-Request-ID"))
		require.Equal(t, "internal-secret", request.Header.Get("X-Internal-Token"))
		require.NoError(t, json.NewDecoder(request.Body).Decode(&received))
		writer.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	err := client.CommitDebit(context.Background(), inventorygateway.DebitCommand{
		CommandID: "command-1", InvoiceID: "invoice-1", Items: []inventorygateway.DebitItem{{ProductID: "product-1", Quantity: 2}},
	})

	require.NoError(t, err)
	require.Equal(t, "command-1", received["commandId"])
	require.Equal(t, "invoice-1", received["invoiceId"])
	items := received["items"].([]any)
	require.Equal(t, "product-1", items[0].(map[string]any)["productId"])
	require.Equal(t, float64(2), items[0].(map[string]any)["quantity"])
}

func TestCommitDebitMapsConflictToTerminalRejection(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Content-Type", "application/problem+json")
		writer.WriteHeader(stdhttp.StatusConflict)
		_, _ = writer.Write([]byte(`{"code":"INSUFFICIENT_STOCK","detail":"Insufficient stock."}`))
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	err := client.CommitDebit(context.Background(), inventorygateway.DebitCommand{})

	var rejected *inventorygateway.RejectedError
	require.ErrorAs(t, err, &rejected)
	require.Equal(t, "INSUFFICIENT_STOCK", rejected.Code)
}

func TestCommitDebitRetriesUnknownClientProblem(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Content-Type", "application/problem+json")
		writer.WriteHeader(stdhttp.StatusConflict)
		_, _ = writer.Write([]byte(`{"code":"ROUTE_CONFLICT","detail":"Unexpected gateway response."}`))
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	err := client.CommitDebit(context.Background(), inventorygateway.DebitCommand{})

	var rejected *inventorygateway.RejectedError
	require.False(t, errors.As(err, &rejected))
	require.ErrorIs(t, err, inventorygateway.ErrUnavailable)
}

func TestFindProductMapsSnapshot(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		require.Equal(t, "/api/inventory/v1/products/product-1", request.URL.Path)
		require.Equal(t, "internal-secret", request.Header.Get("X-Internal-Token"))
		_, _ = writer.Write([]byte(`{"id":"product-1","code":"SKU-1","description":"Keyboard"}`))
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	product, err := client.FindProduct(context.Background(), "product-1")

	require.NoError(t, err)
	require.Equal(t, "SKU-1", product.Code)
	require.Equal(t, "Keyboard", product.Description)
}

func TestFindProductOnlyMapsKnownNotFoundProblem(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Content-Type", "application/problem+json")
		writer.WriteHeader(stdhttp.StatusNotFound)
		_, _ = writer.Write([]byte(`{"code":"ROUTE_NOT_FOUND","detail":"Route not found."}`))
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	_, err := client.FindProduct(context.Background(), "product-1")

	require.NotErrorIs(t, err, inventorygateway.ErrProductNotFound)
	require.ErrorIs(t, err, inventorygateway.ErrUnavailable)
}

func TestFindProductMapsKnownNotFoundProblem(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
		writer.Header().Set("Content-Type", "application/problem+json")
		writer.WriteHeader(stdhttp.StatusNotFound)
		_, _ = writer.Write([]byte(`{"code":"PRODUCT_NOT_FOUND","detail":"Product not found."}`))
	}))
	defer server.Close()
	client := New(server.URL, "internal-secret", server.Client())

	_, err := client.FindProduct(context.Background(), "product-1")

	require.ErrorIs(t, err, inventorygateway.ErrProductNotFound)
}

func TestFindProductEscapesProductPathSegment(t *testing.T) {
	transport := roundTripFunc(func(request *stdhttp.Request) (*stdhttp.Response, error) {
		require.Equal(t, "/api/inventory/v1/products/product%2F..%2Fother", request.URL.EscapedPath())
		return &stdhttp.Response{
			StatusCode: stdhttp.StatusOK,
			Header:     make(stdhttp.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"product","code":"PRD-1","description":"Produto"}`)),
		}, nil
	})
	client := New("http://inventory", "internal-secret", &stdhttp.Client{Transport: transport})

	_, err := client.FindProduct(context.Background(), "product/../other")

	require.NoError(t, err)
}
