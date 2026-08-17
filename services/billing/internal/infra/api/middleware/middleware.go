// Package middleware contains API cross-cutting middleware.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request-id"

// RequestID returns the correlation identifier stored in context.
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

// WithRequestID adds a request identifier to every request and response.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}
		writer.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), requestIDKey, requestID)))
	})
}

// Recover converts unexpected panics to safe responses.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestID := RequestID(request.Context())
				slog.ErrorContext(request.Context(), "panic recovered", "request_id", requestID, "panic", recovered, "stack", string(debug.Stack()))
				writer.Header().Set("Content-Type", "application/problem+json")
				writer.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(writer).Encode(struct {
					Type      string     `json:"type"`
					Title     string     `json:"title"`
					Status    int        `json:"status"`
					Code      string     `json:"code"`
					Detail    string     `json:"detail"`
					TraceID   string     `json:"traceId"`
					Retryable bool       `json:"retryable"`
					Errors    []struct{} `json:"errors"`
				}{
					Type: "https://korp.dev/problems/INTERNAL_ERROR", Title: "Erro interno",
					Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR",
					Detail: "O servidor não conseguiu processar a solicitação. Tente novamente e, se o problema persistir, informe o código de rastreio.", TraceID: requestID,
					Retryable: true, Errors: []struct{}{},
				})
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

// Log emits one structured access log per request.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		wrapped := &responseWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(wrapped, request)
		slog.InfoContext(request.Context(), "http request", "request_id", RequestID(request.Context()), "method", request.Method, "path", request.URL.Path, "status", wrapped.status, "duration_ms", time.Since(started).Milliseconds())
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *responseWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}
