// Package api configures the Inventory Service HTTP boundary.
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/macielcr7/korp-teste/services/inventory/internal/infra/api/httpx"
)

const productsPath = "/api/inventory/v1/products"

// ReadinessChecker verifies whether a required dependency is available.
type ReadinessChecker interface {
	PingContext(ctx context.Context) error
}

// MetricsProvider supplies HTTP instrumentation and the Prometheus exporter.
type MetricsProvider interface {
	Middleware(next http.Handler) http.Handler
	Handler() http.Handler
}

// ProductHandler is the HTTP surface exposed by the product adapter.
type ProductHandler interface {
	Create(response http.ResponseWriter, request *http.Request)
	Get(response http.ResponseWriter, request *http.Request)
	List(response http.ResponseWriter, request *http.Request)
}

// StockCommandHandler is the HTTP surface exposed by the command adapter.
type StockCommandHandler interface {
	Commit(response http.ResponseWriter, request *http.Request)
	Get(response http.ResponseWriter, request *http.Request)
}

// NewPublicRouter builds the public product and operations HTTP boundary.
func NewPublicRouter(
	logger *slog.Logger,
	readiness ReadinessChecker,
	products ProductHandler,
	metrics MetricsProvider,
) http.Handler {
	router := newBaseRouter(logger, metrics)

	router.Get("/health", func(response http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(response, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/ready", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := readiness.PingContext(ctx); err != nil {
			httpx.WriteProblem(response, request, httpx.Problem{
				Type: "urn:korp:problem:not-ready", Title: "Serviço indisponível",
				Status: http.StatusServiceUnavailable, Code: "NOT_READY",
				Detail: "Uma dependência necessária está indisponível.", Retryable: true,
			})
			return
		}
		httpx.WriteJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	})
	router.Handle("/metrics", metrics.Handler())

	router.Post(productsPath, products.Create)
	router.Get(productsPath, products.List)
	router.Get(productsPath+"/{id}", products.Get)

	return router
}

// NewInternalRouter builds the private Billing-to-Inventory HTTP boundary.
func NewInternalRouter(
	logger *slog.Logger,
	products ProductHandler,
	commands StockCommandHandler,
	metrics MetricsProvider,
	internalToken string,
) http.Handler {
	router := newBaseRouter(logger, metrics)
	router.Use(internalTokenAuthentication(internalToken))

	router.Post(productsPath, products.Create)
	router.Get(productsPath, products.List)
	router.Get(productsPath+"/{id}", products.Get)
	router.Post("/internal/inventory/v1/stock-debits", commands.Commit)
	router.Get("/internal/inventory/v1/stock-debits/{commandId}", commands.Get)

	return router
}

func internalTokenAuthentication(expectedToken string) func(http.Handler) http.Handler {
	expectedDigest := sha256.Sum256([]byte(expectedToken))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			providedToken := strings.TrimSpace(request.Header.Get("X-Internal-Token"))
			providedDigest := sha256.Sum256([]byte(providedToken))
			if expectedToken == "" || subtle.ConstantTimeCompare(providedDigest[:], expectedDigest[:]) != 1 {
				response.Header().Set("WWW-Authenticate", `X-Internal-Token`)
				httpx.WriteProblem(response, request, httpx.Problem{
					Type: "urn:korp:problem:unauthorized", Title: "Não autorizado",
					Status: http.StatusUnauthorized, Code: "UNAUTHORIZED",
					Detail: "A credencial interna é inválida ou não foi informada.",
				})
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func newBaseRouter(logger *slog.Logger, metrics MetricsProvider) *chi.Mux {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(metrics.Middleware)
	router.Use(recoverer(logger))
	router.Use(accessLog(logger))
	router.Use(securityHeaders)
	return router
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		id := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		response.Header().Set("X-Request-ID", id)
		next.ServeHTTP(response, request.WithContext(httpx.WithRequestID(request.Context(), id)))
	})
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered", "request_id", httpx.RequestID(request.Context()), "panic", recovered)
					httpx.WriteProblem(response, request, httpx.Problem{
						Type: "urn:korp:problem:internal-error", Title: "Erro interno",
						Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR",
						Detail: "O servidor de estoque não conseguiu processar a solicitação. Tente novamente e, se o problema persistir, informe o código de rastreio.", Retryable: true,
					})
				}
			}()
			next.ServeHTTP(response, request)
		})
	}
}

func accessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			captured := &statusWriter{ResponseWriter: response, status: http.StatusOK}
			next.ServeHTTP(captured, request)
			logger.Info("http request",
				"request_id", httpx.RequestID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", captured.status,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(response, request)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}
