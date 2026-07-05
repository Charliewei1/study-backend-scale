package middleware

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/study-backend-scale/shortlink/internal/metrics"
	"golang.org/x/time/rate"
)

const (
	rateLimitCleanupAfter = 10 * time.Minute
	rateLimitMaxClients   = 10_000
)

type RateLimiter struct {
	rps        rate.Limit
	burst      int
	trustProxy bool

	mu          sync.Mutex
	clients     map[string]*clientLimiter
	lastCleanup time.Time
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(rps float64, burst int) (*RateLimiter, error) {
	if rps <= 0 {
		return nil, errors.New("rate limit rps must be greater than 0")
	}
	if burst <= 0 {
		return nil, errors.New("rate limit burst must be greater than 0")
	}

	return &RateLimiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		clients: make(map[string]*clientLimiter),
	}, nil
}

func RateLimitFromEnv() (func(http.Handler) http.Handler, error) {
	rpsValue := os.Getenv("RATE_LIMIT_RPS")
	burstValue := os.Getenv("RATE_LIMIT_BURST")
	if rpsValue == "" && burstValue == "" {
		return func(next http.Handler) http.Handler { return next }, nil
	}
	if rpsValue == "" || burstValue == "" {
		return nil, errors.New("RATE_LIMIT_RPS and RATE_LIMIT_BURST must be set together")
	}

	rps, err := strconv.ParseFloat(rpsValue, 64)
	if err != nil {
		return nil, fmt.Errorf("parse RATE_LIMIT_RPS: %w", err)
	}
	burst, err := strconv.Atoi(burstValue)
	if err != nil {
		return nil, fmt.Errorf("parse RATE_LIMIT_BURST: %w", err)
	}

	limiter, err := NewRateLimiter(rps, burst)
	if err != nil {
		return nil, err
	}
	limiter.trustProxy = os.Getenv("TRUST_PROXY") == "true"
	return limiter.Middleware, nil
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := l.limiterFor(l.clientIP(r), time.Now())

		if client.Allow() {
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.burst))
			w.Header().Set("X-RateLimit-Remaining", formatRemaining(client.Tokens()))
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.burst))
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Retry-After", retryAfter(client))
		// Metrics wraps ServeMux directly so it can read r.Pattern after routing.
		// Rate-limit rejections happen before routing, so they are counted with a
		// dedicated low-cardinality counter instead of http_requests_total.
		metrics.RecordRateLimited()
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	})
}

func (l *RateLimiter) limiterFor(ip string, now time.Time) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanup(now)

	if existing, ok := l.clients[ip]; ok {
		existing.lastSeen = now
		return existing.limiter
	}
	if len(l.clients) >= rateLimitMaxClients {
		l.evictOldest()
	}

	limiter := rate.NewLimiter(l.rps, l.burst)
	l.clients[ip] = &clientLimiter{
		limiter:  limiter,
		lastSeen: now,
	}
	return limiter
}

func (l *RateLimiter) cleanup(now time.Time) {
	if now.Sub(l.lastCleanup) < time.Minute {
		return
	}
	l.lastCleanup = now

	for ip, client := range l.clients {
		if now.Sub(client.lastSeen) > rateLimitCleanupAfter {
			delete(l.clients, ip)
		}
	}
}

func (l *RateLimiter) evictOldest() {
	var oldestIP string
	var oldestSeen time.Time
	for ip, client := range l.clients {
		if oldestIP == "" || client.lastSeen.Before(oldestSeen) {
			oldestIP = ip
			oldestSeen = client.lastSeen
		}
	}
	if oldestIP != "" {
		delete(l.clients, oldestIP)
	}
}

func (l *RateLimiter) clientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); l.trustProxy && forwardedFor != "" {
		// X-Forwarded-For is client-controlled unless a trusted reverse proxy
		// overwrites it. Only TRUST_PROXY=true deployments use it; direct internet
		// traffic falls back to RemoteAddr so clients cannot rotate spoofed headers
		// to bypass a per-IP limiter.
		parts := strings.Split(forwardedFor, ",")
		if addr, err := netip.ParseAddr(strings.TrimSpace(parts[0])); err == nil {
			return addr.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String()
	}
	return host
}

func retryAfter(limiter *rate.Limiter) string {
	reservation := limiter.Reserve()
	if !reservation.OK() {
		return "1"
	}
	defer reservation.Cancel()

	seconds := int(math.Ceil(reservation.Delay().Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func formatRemaining(tokens float64) string {
	remaining := int(math.Floor(tokens))
	if remaining < 0 {
		remaining = 0
	}
	return strconv.Itoa(remaining)
}
