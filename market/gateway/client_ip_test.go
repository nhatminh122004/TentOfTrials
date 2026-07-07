package gateway

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "host port remote address",
			remoteAddr: "203.0.113.10:54321",
			want:       "203.0.113.10",
		},
		{
			name:       "bare ip remote address",
			remoteAddr: "203.0.113.11",
			want:       "203.0.113.11",
		},
		{
			name:       "malformed remote address fallback",
			remoteAddr: "not a valid remote address",
			want:       "not a valid remote address",
		},
		{
			name:       "x forwarded for takes first address",
			remoteAddr: "203.0.113.12:54321",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.10, 198.51.100.11",
			},
			want: "198.51.100.10",
		},
		{
			name:       "x real ip is used when forwarded for is absent",
			remoteAddr: "203.0.113.13:54321",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.20",
			},
			want: "198.51.100.20",
		},
		{
			name:       "x forwarded for wins over x real ip",
			remoteAddr: "203.0.113.14:54321",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.30",
				"X-Real-IP":       "198.51.100.31",
			},
			want: "198.51.100.30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for name, value := range tt.headers {
				req.Header.Set(name, value)
			}

			if got := getClientIP(req); got != tt.want {
				t.Fatalf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimitMiddlewareUsesResolvedClientIP(t *testing.T) {
	handler := RateLimitMiddleware(0.0001, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "203.0.113.10:1000"
	req1.Header.Set("X-Forwarded-For", "198.51.100.50")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusNoContent)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "203.0.113.11:1000"
	req2.Header.Set("X-Forwarded-For", "198.51.100.50")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "203.0.113.12:1000"
	req3.Header.Set("X-Forwarded-For", "198.51.100.51")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("different client status = %d, want %d", rec3.Code, http.StatusNoContent)
	}
}

func TestLoggingMiddlewareUsesResolvedClientIP(t *testing.T) {
	var buf bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.10:1000"
	req.Header.Set("X-Real-IP", "198.51.100.60")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	line, err := io.ReadAll(&buf)
	if err != nil {
		t.Fatalf("reading log buffer: %v", err)
	}
	if !strings.Contains(string(line), "[198.51.100.60]") {
		t.Fatalf("log line %q does not contain resolved client IP", string(line))
	}
}
