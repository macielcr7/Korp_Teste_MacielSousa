package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetricsUseResolvedRouteMethodAndStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewHTTPMetrics(registry)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(metrics.Middleware)
	router.Get("/invoices/{id}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/invoices/42", nil))

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, float64(1), testutil.ToFloat64(metrics.requests.WithLabelValues("/invoices/{id}", http.MethodGet, "202")))
	families, err := registry.Gather()
	require.NoError(t, err)
	require.Contains(t, metricFamilyNames(families), "billing_http_request_duration_seconds")
}

func metricFamilyNames(families []*dto.MetricFamily) []string {
	names := make([]string, 0, len(families))
	for _, family := range families {
		names = append(names, family.GetName())
	}
	return names
}
