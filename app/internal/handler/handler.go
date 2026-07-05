// Package handler は HTTP API とアプリ内部の部品をつなぎます。
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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
	mux.HandleFunc("GET /api/links/{code}", h.getLink)
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

type linkResponse struct {
	Code     string `json:"code"`
	URL      string `json:"url"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateURL(req.URL); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	code := h.shortener.Next()
	h.store.Save(code, req.URL)
	shortURL := h.shortURL(code)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", shortURL)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createLinkResponse{
		Code:     code,
		ShortURL: shortURL,
	})
}

func (h *Handler) getLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	originalURL, ok := h.store.Load(code)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "link not found")
		return
	}

	writeJSON(w, http.StatusOK, linkResponse{
		Code:     code,
		URL:      originalURL,
		ShortURL: h.shortURL(code),
	})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	originalURL, ok := h.store.Load(code)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "link not found")
		return
	}

	http.Redirect(w, r, originalURL, http.StatusFound)
}

func (h *Handler) shortURL(code string) string {
	return h.baseURL + "/" + code
}

func validateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("url is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("url must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("url host is required")
	}
	return nil
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
