package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	metricsprometheus "github.com/macielcr7/korp-teste/services/inventory/internal/infra/observability/prometheus"
)

type readinessStub struct{ err error }

func (stub readinessStub) PingContext(context.Context) error { return stub.err }

type productHandlerStub struct{}

func (productHandlerStub) Create(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusCreated)
}
func (productHandlerStub) Get(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}
func (productHandlerStub) List(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}

type commandHandlerStub struct{}

func (commandHandlerStub) Commit(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}
func (commandHandlerStub) Get(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}

func TestHealth(t *testing.T) {
	router := testPublicRouter(readinessStub{})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotEmpty(t, response.Header().Get("X-Request-ID"))
	assert.JSONEq(t, `{"status":"ok"}`, response.Body.String())
}

func TestReadyReturnsServiceUnavailable(t *testing.T) {
	router := testPublicRouter(readinessStub{err: errors.New("database unavailable")})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	request.Header.Set("X-Request-ID", "request-123")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Equal(t, "request-123", response.Header().Get("X-Request-ID"))
	assert.Contains(t, response.Body.String(), `"code":"NOT_READY"`)
	assert.Contains(t, response.Body.String(), `"traceId":"request-123"`)
}

func TestMetricsExposesHTTPRouteCountersAndDuration(t *testing.T) {
	router := testPublicRouter(readinessStub{})
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(httptest.NewRecorder(), healthRequest)
	productsRequest := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products", nil)
	router.ServeHTTP(httptest.NewRecorder(), productsRequest)
	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, metricsRequest)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `inventory_http_requests_total{method="GET",route="/health",status="200"} 1`)
	assert.Contains(t, response.Body.String(), `inventory_http_request_duration_seconds_count{method="GET",route="/health",status="200"} 1`)
	assert.Contains(t, response.Body.String(), `inventory_http_requests_total{method="GET",route="/api/inventory/v1/products",status="200"} 1`)
	assert.NotContains(t, response.Body.String(), `inventory_http_requests_total{method="GET",route="unmatched",status="200"}`)
}

func TestPublicRouterDoesNotExposeInternalStockCommands(t *testing.T) {
	router := testPublicRouter(readinessStub{})
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInternalRouterExposesStockCommandsAndProductLookup(t *testing.T) {
	logger, metrics := testDependencies()
	router := NewInternalRouter(logger, productHandlerStub{}, commandHandlerStub{}, metrics, "internal-secret")

	stockRequest := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", nil)
	stockRequest.Header.Set("X-Internal-Token", "internal-secret")
	stockResponse := httptest.NewRecorder()
	router.ServeHTTP(stockResponse, stockRequest)
	productRequest := httptest.NewRequest(http.MethodGet, "/api/inventory/v1/products/57c30a21-b548-4bc1-b4c2-39dc94e20829", nil)
	productRequest.Header.Set("X-Internal-Token", "internal-secret")
	productResponse := httptest.NewRecorder()
	router.ServeHTTP(productResponse, productRequest)

	assert.Equal(t, http.StatusOK, stockResponse.Code)
	assert.Equal(t, http.StatusOK, productResponse.Code)
}

func TestInternalRouterRejectsMissingOrInvalidToken(t *testing.T) {
	logger, metrics := testDependencies()
	router := NewInternalRouter(logger, productHandlerStub{}, commandHandlerStub{}, metrics, "internal-secret")

	for _, token := range []string{"", "wrong-secret"} {
		request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", nil)
		if token != "" {
			request.Header.Set("X-Internal-Token", token)
		}
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
		assert.Contains(t, response.Body.String(), `"code":"UNAUTHORIZED"`)
	}
}

func TestInternalRouterRejectsEmptyConfiguredToken(t *testing.T) {
	logger, metrics := testDependencies()
	router := NewInternalRouter(logger, productHandlerStub{}, commandHandlerStub{}, metrics, "")
	request := httptest.NewRequest(http.MethodPost, "/internal/inventory/v1/stock-debits", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func testPublicRouter(readiness ReadinessChecker) http.Handler {
	logger, metrics := testDependencies()
	return NewPublicRouter(logger, readiness, productHandlerStub{}, metrics)
}

func testDependencies() (*slog.Logger, *metricsprometheus.Metrics) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := promclient.NewRegistry()
	metrics := metricsprometheus.New(registry, registry)
	return logger, metrics
}
