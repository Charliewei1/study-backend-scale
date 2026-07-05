package main

import (
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

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	h := handler.New(shortener.New(), storage.NewMemoryStore(), baseURL)

	addr := ":" + port
	log.Printf("shortlink server listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
