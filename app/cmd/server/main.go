package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/study-backend-scale/shortlink/internal/analytics"
	"github.com/study-backend-scale/shortlink/internal/cache"
	"github.com/study-backend-scale/shortlink/internal/handler"
	"github.com/study-backend-scale/shortlink/internal/middleware"
	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const shutdownTimeout = 5 * time.Second

func main() {
	logger := newLogger()
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, closeStore := newStore()
	defer closeStore()

	var cacheStats cache.StatsProvider
	store, closeCache, cacheStats := withRedisCache(store)
	defer closeCache()

	clicks := analytics.New(store, 1024)

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	appHandler := handler.New(shortener.New(), store, baseURL, clicks, cacheStats)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("/", appHandler)
	h := middleware.RequestLogger(logger)(middleware.Metrics(mux))

	addr := ":" + port
	server := &http.Server{
		Addr:    addr,
		Handler: h,
	}
	debugServer := newDebugServer()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		slog.Info("shortlink server listening", "addr", addr)
		errCh <- server.ListenAndServe()
	}()
	if debugServer != nil {
		go func() {
			slog.Info("debug server listening", "addr", debugServer.Addr)
			errCh <- debugServer.ListenAndServe()
		}()
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fatal("http server failed", "error", err)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown http server", "error", err)
		}
		if debugServer != nil {
			if err := debugServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("shutdown debug server", "error", err)
			}
		}
		cancel()
	}

	flushCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := clicks.Close(flushCtx); err != nil {
		slog.Error("flush analytics", "error", err)
	}
	cancel()
}

func newLogger() *slog.Logger {
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func withRedisCache(store storage.Storage) (storage.Storage, func(), cache.StatsProvider) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		return store, func() {}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})
	cached := cache.NewRedisStore(store, client)
	slog.Info("redis cache enabled", "addr", redisAddr)

	return cached, func() {
		if err := client.Close(); err != nil {
			slog.Error("close redis client", "error", err)
		}
	}, cached
}

func newDebugServer() *http.Server {
	addr := os.Getenv("DEBUG_ADDR")
	if addr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr: addr,
		// pprof は内部状態を見せるため、本番 API と同じポートではなく
		// DEBUG_ADDR の別ポートで明示的に有効化したときだけ公開する。
		Handler: mux,
	}
}

func newStore() (storage.Storage, func()) {
	switch os.Getenv("STORAGE") {
	case "", "memory":
		return storage.NewMemoryStore(), func() {}
	case "sqlite":
		path := os.Getenv("SQLITE_PATH")
		if path == "" {
			path = "data.db"
		}

		store, err := storage.NewSQLiteStore(context.Background(), path)
		if err != nil {
			fatal("initialize sqlite storage", "error", err)
		}
		return store, func() {
			if err := store.Close(); err != nil {
				slog.Error("close sqlite storage", "error", err)
			}
		}
	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			fatal("DATABASE_URL is required when STORAGE=postgres")
		}

		store, err := storage.NewPostgresStore(context.Background(), databaseURL)
		if err != nil {
			fatal("initialize postgres storage", "error", err)
		}
		return store, func() {
			store.Close()
		}
	default:
		fatal("unsupported STORAGE; want memory, sqlite, or postgres", "storage", os.Getenv("STORAGE"))
		return nil, func() {}
	}
}
