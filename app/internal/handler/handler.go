// Package handler は HTTP API とアプリ内部の部品をつなぎます。
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

// Handler は URL 短縮 API の依存関係をまとめた型です。
type Handler struct {
	shortener *shortener.Shortener
	store     *storage.MemoryStore
	baseURL   string
}

// New は Go 1.22 で追加された ServeMux の "METHOD /path" パターンを使って
// ルーティングを登録します。
func New(shortener *shortener.Shortener, store *storage.MemoryStore, baseURL string) http.Handler {
	h := &Handler{
		shortener: shortener,
		store:     store,
		baseURL:   strings.TrimRight(baseURL, "/"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("POST /api/links", h.createLink)
	mux.HandleFunc("GET /{code}", h.redirect)
	return mux
}

type createLinkRequest struct {
	URL string `json:"url"`
}

type createLinkResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	code := h.shortener.Next()
	h.store.Save(code, req.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createLinkResponse{
		Code:     code,
		ShortURL: h.baseURL + "/" + code,
	})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	url, ok := h.store.Load(code)
	if !ok {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
