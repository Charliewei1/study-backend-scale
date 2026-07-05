package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

const DefaultRequestTimeout = 5 * time.Second

func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestTimeoutFromEnv() (time.Duration, error) {
	value := os.Getenv("REQUEST_TIMEOUT")
	if value == "" {
		return DefaultRequestTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse REQUEST_TIMEOUT: %w", err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("REQUEST_TIMEOUT must be greater than 0")
	}
	return timeout, nil
}
