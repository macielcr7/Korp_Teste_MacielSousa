// Package api defines the Billing HTTP transport.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	closurehandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/closureoperation"
	invoicehandler "github.com/macielcr7/korp-teste/services/billing/internal/infra/api/handler/billing/invoice"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/api/middleware"
	"github.com/macielcr7/korp-teste/services/billing/internal/infra/observability"
)

// NewRouter builds the versioned Billing HTTP router.
func NewRouter(invoices *invoicehandler.Handler, operations *closurehandler.Handler, ready func(*http.Request) error, httpMetrics *observability.HTTPMetrics, gatherer prometheus.Gatherer) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.WithRequestID, httpMetrics.Middleware, middleware.Log, middleware.Recover)
	router.Get("/health", statusHandler(http.StatusOK, "ok"))
	router.Get("/healthz", statusHandler(http.StatusOK, "ok"))
	router.Get("/ready", readinessHandler(ready))
	router.Get("/readyz", readinessHandler(ready))
	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	router.Route("/api/billing/v1", func(router chi.Router) {
		router.Post("/invoices", invoices.Create)
		router.Get("/invoices", invoices.List)
		router.Get("/invoices/{id}", invoices.Get)
		router.Post("/invoices/{id}/close", invoices.RequestClosure)
		router.Get("/invoices/{id}/printable", invoices.Printable)
		router.Get("/closure-operations/{id}", operations.Get)
	})
	return router
}

func statusHandler(status int, value string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]string{"status": value})
	}
}

func readinessHandler(ready func(*http.Request) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := ready(request); err != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(writer).Encode(map[string]string{"status": "unavailable"})
			return
		}
		statusHandler(http.StatusOK, "ready")(writer, request)
	}
}
