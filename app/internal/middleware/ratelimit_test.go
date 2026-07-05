package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/study-backend-scale/shortlink/internal/metrics"
)

func TestRateLimitRejectsRequestsOverBurst(t *testing.T) {
	limiter, err := NewRateLimiter(1, 2)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}

	var handled int
	h := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handled++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newRateLimitRequest())
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d; body=%s", i+1, rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newRateLimitRequest())

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if handled != 2 {
		t.Fatalf("handled requests = %d, want 2", handled)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
		t.Fatalf("X-RateLimit-Limit = %q, want 2", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want 0", got)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatal("Retry-After is empty")
	}
}

func TestRateLimitRecordsDedicatedMetric(t *testing.T) {
	limiter, err := NewRateLimiter(1, 1)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}
	h := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	before := testutil.ToFloat64(metrics.RateLimitedTotal)

	h.ServeHTTP(httptest.NewRecorder(), newRateLimitRequest())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newRateLimitRequest())

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := testutil.ToFloat64(metrics.RateLimitedTotal) - before; got != 1 {
		t.Fatalf("rate_limited_total delta = %v, want 1", got)
	}
}

func TestRateLimitRecoversAfterTokensRefill(t *testing.T) {
	limiter, err := NewRateLimiter(100, 1)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}
	h := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	h.ServeHTTP(first, newRateLimitRequest())
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, newRateLimitRequest())
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d", blocked.Code, http.StatusTooManyRequests)
	}

	time.Sleep(25 * time.Millisecond)

	recovered := httptest.NewRecorder()
	h.ServeHTTP(recovered, newRateLimitRequest())
	if recovered.Code != http.StatusOK {
		t.Fatalf("recovered status = %d, want %d; body=%s", recovered.Code, http.StatusOK, recovered.Body.String())
	}
}

func TestRateLimitTrustsXForwardedForOnlyWhenConfigured(t *testing.T) {
	limiter, err := NewRateLimiter(1, 1)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}

	req := newRateLimitRequest()
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 198.51.100.8")
	if got := limiter.clientIP(req); got != "203.0.113.10" {
		t.Fatalf("clientIP without TRUST_PROXY = %q, want RemoteAddr", got)
	}

	limiter.trustProxy = true
	if got := limiter.clientIP(req); got != "198.51.100.7" {
		t.Fatalf("clientIP with TRUST_PROXY = %q, want first XFF IP", got)
	}

	req.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := limiter.clientIP(req); got != "203.0.113.10" {
		t.Fatalf("clientIP with invalid XFF = %q, want RemoteAddr", got)
	}
}

func TestRateLimitCleansUpOldClients(t *testing.T) {
	limiter, err := NewRateLimiter(1, 1)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)

	limiter.limiterFor("old", now.Add(-rateLimitCleanupAfter-time.Second))
	limiter.limiterFor("new", now)

	if _, ok := limiter.clients["old"]; ok {
		t.Fatal("old client was not cleaned up")
	}
	if _, ok := limiter.clients["new"]; !ok {
		t.Fatal("new client was not recorded")
	}
}

func TestRateLimitCapsClientMap(t *testing.T) {
	limiter, err := NewRateLimiter(1, 1)
	if err != nil {
		t.Fatalf("NewRateLimiter returned error: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	limiter.lastCleanup = now

	for i := 0; i < rateLimitMaxClients+1; i++ {
		limiter.limiterFor(fmt.Sprintf("client-%d", i), now.Add(time.Duration(i)*time.Millisecond))
	}

	if got := len(limiter.clients); got != rateLimitMaxClients {
		t.Fatalf("client map length = %d, want %d", got, rateLimitMaxClients)
	}
	if _, ok := limiter.clients["client-0"]; ok {
		t.Fatal("oldest client was not evicted")
	}
}

func newRateLimitRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	return req
}
