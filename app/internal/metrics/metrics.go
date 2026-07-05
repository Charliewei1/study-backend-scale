// Package metrics owns the Prometheus collectors exported by the service.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled by the application.",
		},
		[]string{"method", "route", "status"},
	)

	HTTPRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route"},
	)

	CacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits.",
		},
	)

	CacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses.",
		},
	)
)

func RecordHTTPRequest(method, route string, status int, duration time.Duration) {
	HTTPRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	HTTPRequestDurationSeconds.WithLabelValues(route).Observe(duration.Seconds())
}

func RecordCacheHit() {
	CacheHitsTotal.Inc()
}

func RecordCacheMiss() {
	CacheMissesTotal.Inc()
}
