// Package prometheus provides Inventory Service Prometheus instrumentation.
package prometheus

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatchedRoute = "unmatched"

// Metrics owns the Inventory Service metric collectors and HTTP exporter.
type Metrics struct {
	gatherer       promclient.Gatherer
	httpRequests   *promclient.CounterVec
	httpDuration   *promclient.HistogramVec
	stockDebitRuns *promclient.CounterVec
}

// New registers and returns the Inventory Service metric collectors.
func New(registerer promclient.Registerer, gatherer promclient.Gatherer) *Metrics {
	metrics := &Metrics{
		gatherer: gatherer,
		httpRequests: promclient.NewCounterVec(promclient.CounterOpts{
			Namespace: "inventory",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of Inventory Service HTTP requests.",
		}, []string{"route", "method", "status"}),
		httpDuration: promclient.NewHistogramVec(promclient.HistogramOpts{
			Namespace: "inventory",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Inventory Service HTTP request duration in seconds.",
			Buckets:   promclient.DefBuckets,
		}, []string{"route", "method", "status"}),
		stockDebitRuns: promclient.NewCounterVec(promclient.CounterOpts{
			Namespace: "inventory",
			Subsystem: "stock_debit",
			Name:      "operations_total",
			Help:      "Total number of stock debit operations by result.",
		}, []string{"result"}),
	}
	registerer.MustRegister(metrics.httpRequests, metrics.httpDuration, metrics.stockDebitRuns)
	return metrics
}

// Handler exposes metrics using the Prometheus text protocol.
func (metrics *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.gatherer, promhttp.HandlerOpts{})
}

// Middleware records HTTP request count and duration using bounded-cardinality labels.
func (metrics *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		captured := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(captured, request)

		route := chi.RouteContext(request.Context()).RoutePattern()
		if route == "" {
			route = unmatchedRoute
		}
		status := strconv.Itoa(captured.status)
		metrics.httpRequests.WithLabelValues(route, request.Method, status).Inc()
		metrics.httpDuration.WithLabelValues(route, request.Method, status).Observe(time.Since(startedAt).Seconds())
	})
}

func (metrics *Metrics) observeStockDebit(result string) {
	metrics.stockDebitRuns.WithLabelValues(result).Inc()
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (recorder *statusRecorder) WriteHeader(status int) {
	if recorder.wroteHeader {
		return
	}
	recorder.wroteHeader = true
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if !recorder.wroteHeader {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(body)
}
