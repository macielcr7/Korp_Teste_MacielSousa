// Package observability exposes Billing metrics without leaking monitoring concerns into inner layers.
package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics records bounded-cardinality request metrics.
type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHTTPMetrics creates and registers the HTTP metrics.
func NewHTTPMetrics(registerer prometheus.Registerer) (*HTTPMetrics, error) {
	metrics := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "billing",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of Billing HTTP requests.",
		}, []string{"route", "method", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "billing",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Billing HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"route", "method", "status"}),
	}
	if err := registerer.Register(metrics.requests); err != nil {
		return nil, fmt.Errorf("register HTTP request counter: %w", err)
	}
	if err := registerer.Register(metrics.duration); err != nil {
		return nil, fmt.Errorf("register HTTP duration histogram: %w", err)
	}
	return metrics, nil
}

// Middleware records request count and duration after Chi resolves the route pattern.
func (metrics *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(wrapped, request)

		route := chi.RouteContext(request.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(wrapped.status)
		metrics.requests.WithLabelValues(route, request.Method, status).Inc()
		metrics.duration.WithLabelValues(route, request.Method, status).Observe(time.Since(started).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.status = status
	writer.wroteHeader = true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(payload []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(payload)
}
