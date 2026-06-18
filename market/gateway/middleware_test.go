package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitMiddlewareAllowsTraffic(t *testing.T) {
	handler := RateLimitMiddleware(DefaultRateLimitPerSecond, DefaultRateLimitBurst)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, requestFromIP("203.0.113.10"))

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected allowed request, got status %d", response.Code)
	}
	if got := response.Header().Get(RateLimitLimitHeader); got != "120" {
		t.Fatalf("expected default per-minute limit header 120, got %q", got)
	}
	if got := response.Header().Get(RateLimitRemainingHeader); got == "" {
		t.Fatal("expected remaining limit header")
	}
	if got := response.Header().Get(RateLimitRetryAfterHeader); got != "" {
		t.Fatalf("did not expect retry header on allowed request, got %q", got)
	}
}

func TestRateLimitMiddlewareBlocksTraffic(t *testing.T) {
	handler := RateLimitMiddleware(DefaultRateLimitPerSecond, 1)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, requestFromIP("203.0.113.11"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected first request through, got status %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, requestFromIP("203.0.113.11"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got status %d", second.Code)
	}
	if got := second.Header().Get(RateLimitRetryAfterHeader); got == "" {
		t.Fatal("expected retry-after header on limited request")
	}

	var body map[string]interface{}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body: %v", err)
	}
	if got := body["error"]; got != "rate_limit_exceeded" {
		t.Fatalf("expected rate_limit_exceeded body, got %#v", body)
	}
}

func TestRateLimitMiddlewareUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv(EnvRateLimitPerMinute, "2")
	t.Setenv(EnvRateLimitBurst, "1")

	handler := RateLimitMiddleware(999, 999)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, requestFromIP("203.0.113.12"))
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected first request through, got status %d", first.Code)
	}
	if got := first.Header().Get(RateLimitLimitHeader); got != "2" {
		t.Fatalf("expected env per-minute limit header 2, got %q", got)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, requestFromIP("203.0.113.12"))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected env burst override to block second request, got status %d", second.Code)
	}
}

func requestFromIP(ip string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/market/ticker?symbol=BTC", nil)
	request.RemoteAddr = ip + ":12345"
	return request
}
