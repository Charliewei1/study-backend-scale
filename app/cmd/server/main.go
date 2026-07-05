package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/study-backend-scale/shortlink/internal/analytics"
	"github.com/study-backend-scale/shortlink/internal/handler"
	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const shutdownTimeout = 5 * time.Second

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, closeStore := newStore()
	defer closeStore()

	clicks := analytics.New(store, 1024)

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	h := handler.New(shortener.New(), store, baseURL, clicks)

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
		log.Printf("shortlink server listening on %s", addr)
		errCh <- server.ListenAndServe()
	}()
	if debugServer != nil {
		go func() {
			log.Printf("debug server listening on %s", debugServer.Addr)
			errCh <- debugServer.ListenAndServe()
		}()
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown http server: %v", err)
		}
		if debugServer != nil {
			if err := debugServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("shutdown debug server: %v", err)
			}
		}
		cancel()
	}

	flushCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := clicks.Close(flushCtx); err != nil {
		log.Printf("flush analytics: %v", err)
	}
	cancel()
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
			log.Fatalf("initialize sqlite storage: %v", err)
		}
		return store, func() {
			if err := store.Close(); err != nil {
				log.Printf("close sqlite storage: %v", err)
			}
		}
	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required when STORAGE=postgres")
		}

		store, err := storage.NewPostgresStore(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("initialize postgres storage: %v", err)
		}
		return store, func() {
			store.Close()
		}
	default:
		log.Fatalf("unsupported STORAGE %q; want memory, sqlite, or postgres", os.Getenv("STORAGE"))
		return nil, func() {}
	}
}
