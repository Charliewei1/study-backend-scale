package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/study-backend-scale/shortlink/internal/handler"
	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, closeStore := newStore()
	defer closeStore()

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	h := handler.New(shortener.New(), store, baseURL)

	addr := ":" + port
	log.Printf("shortlink server listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
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
	default:
		log.Fatalf("unsupported STORAGE %q; want memory or sqlite", os.Getenv("STORAGE"))
		return nil, func() {}
	}
}
