// Package middleware provides HTTP middleware used by the server.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/study-backend-scale/shortlink/internal/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skipRequestInstrumentation(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := newStatusResponseWriter(w)

		next.ServeHTTP(rec, r)

		metrics.RecordHTTPRequest(r.Method, routeLabel(r), rec.status, time.Since(start))
	})
}

func routeLabel(r *http.Request) string {
	pattern := r.Pattern
	if pattern == "" {
		return "unknown"
	}

	fields := strings.Fields(pattern)
	if len(fields) == 2 {
		pattern = fields[1]
	}

	if pattern == "/{code}" {
		// Do not label redirects with the raw short code. Every code would create
		// its own Prometheus series, so all short-link redirects are grouped here.
		return "redirect"
	}

	return pattern
}
