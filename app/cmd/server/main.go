package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("shortlink server listening on %s", addr)
		errCh <- server.ListenAndServe()
	}()

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
		cancel()
	}

	flushCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := clicks.Close(flushCtx); err != nil {
		log.Printf("flush analytics: %v", err)
	}
	cancel()
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
	default:
		log.Fatalf("unsupported STORAGE %q; want memory or sqlite", os.Getenv("STORAGE"))
		return nil, func() {}
	}
}
