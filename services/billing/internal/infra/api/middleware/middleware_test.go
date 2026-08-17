package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverReturnsCorrelatedProblemDetails(t *testing.T) {
	t.Parallel()
	const requestID = "8c8ccb26-dca9-4378-b580-5428b45ff3a6"
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set("X-Request-ID", requestID)
	response := httptest.NewRecorder()
	handler := WithRequestID(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("unexpected failure")
	})))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if got := response.Header().Get("X-Request-ID"); got != requestID {
		t.Fatalf("X-Request-ID = %q, want %q", got, requestID)
	}
	var problem struct {
		Code    string `json:"code"`
		TraceID string `json:"traceId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem details: %v", err)
	}
	if problem.Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %q, want INTERNAL_ERROR", problem.Code)
	}
	if problem.TraceID != requestID {
		t.Fatalf("traceId = %q, want %q", problem.TraceID, requestID)
	}
}
