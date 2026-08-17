// Package httpx contains shared HTTP transport helpers.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type requestIDKey struct{}

// FieldError associates a validation message with one request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem is an RFC 9457-compatible API error document.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance,omitempty"`
	TraceID   string       `json:"traceId,omitempty"`
	Retryable bool         `json:"retryable"`
	CommandID string       `json:"commandId,omitempty"`
	Errors    []FieldError `json:"errors"`
}

// WithRequestID stores a correlation ID in the request context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the request correlation ID.
func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// DecodeJSON strictly decodes exactly one JSON object.
func DecodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("request body must contain exactly one JSON object")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// WriteJSON writes a JSON response.
func WriteJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

// WriteProblem writes an application/problem+json response.
func WriteProblem(response http.ResponseWriter, request *http.Request, problem Problem) {
	problem.Status = normalizedStatus(problem.Status)
	problem.Instance = request.URL.Path
	problem.TraceID = RequestID(request.Context())
	if problem.Errors == nil {
		problem.Errors = make([]FieldError, 0)
	}
	response.Header().Set("Content-Type", "application/problem+json")
	response.WriteHeader(problem.Status)
	_ = json.NewEncoder(response).Encode(problem)
}

func normalizedStatus(status int) int {
	if status < 400 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
}
