package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
