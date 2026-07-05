package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/study-backend-scale/shortlink/internal/metrics"
)

func TestMetricsMiddlewareCountsRequests(t *testing.T) {
	appMux := http.NewServeMux()
	appMux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/articles/1", http.StatusFound)
	})
	rootMux := http.NewServeMux()
	rootMux.Handle("/", appMux)
	h := Metrics(rootMux)

	counter := metrics.HTTPRequestsTotal.WithLabelValues("GET", "redirect", "302")
	before := testutil.ToFloat64(counter)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Fatalf("counter delta = %v, want 1", got)
	}
}

func TestMetricsSeesRouteWhenRequestTimeoutIsOuter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/articles/1", http.StatusFound)
	})
	h := RequestTimeout(time.Second)(Metrics(mux))

	counter := metrics.HTTPRequestsTotal.WithLabelValues("GET", "redirect", "302")
	before := testutil.ToFloat64(counter)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Fatalf("counter delta = %v, want 1", got)
	}
}

func TestRequestInstrumentationSkipsMetricsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := RequestLogger(logger)(Metrics(mux))

	counter := metrics.HTTPRequestsTotal.WithLabelValues("GET", "/metrics", "200")
	before := testutil.ToFloat64(counter)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := testutil.ToFloat64(counter) - before; got != 0 {
		t.Fatalf("counter delta = %v, want 0", got)
	}
	if strings.Contains(logs.String(), "http request") {
		t.Fatalf("request log was written for /metrics: %s", logs.String())
	}
}
