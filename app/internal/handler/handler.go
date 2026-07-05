// Package handler は HTTP API とアプリ内部の部品をつなぎます。
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/study-backend-scale/shortlink/internal/cache"
	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const createLinkMaxAttempts = 5
const createLinkMaxBodyBytes = 64 * 1024

// Handler は URL 短縮 API の依存関係をまとめた型です。
type Handler struct {
	shortener *shortener.Shortener
	store     storage.Storage
	baseURL   string
	clicks    clickRecorder
	cache     cache.StatsProvider
}

type clickRecorder interface {
	Record(code string) bool
}

// New は Go 1.22 で追加された ServeMux の "METHOD /path" パターンを使って
// ルーティングを登録します。
func New(shortener *shortener.Shortener, store storage.Storage, baseURL string, clicks clickRecorder, cacheStats ...cache.StatsProvider) http.Handler {
	h := &Handler{
		shortener: shortener,
		store:     store,
		baseURL:   strings.TrimRight(baseURL, "/"),
		clicks:    clicks,
	}
	if len(cacheStats) > 0 {
		h.cache = cacheStats[0]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /readyz", h.readyz)
	mux.HandleFunc("GET /api/cache/stats", h.getCacheStats)
	mux.HandleFunc("POST /api/links", h.createLink)
	mux.HandleFunc("GET /api/links/{code}/stats", h.getLinkStats)
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

type linkStatsResponse struct {
	Code   string `json:"code"`
	URL    string `json:"url"`
	Clicks int64  `json:"clicks"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	// /healthz は liveness 用です。プロセスが応答できるかだけを見て、
	// DB などの外部依存には触りません。依存先障害で再起動ループに入るのを避けます。
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	// /readyz は readiness 用です。リクエストを受けてよい状態かを判定するため、
	// storage に軽く ping します。失敗時は Service の転送先から外して再起動はしません。
	if err := h.store.Ping(r.Context()); err != nil {
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) getCacheStats(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		writeJSON(w, http.StatusOK, cache.Stats{})
		return
	}

	writeJSON(w, http.StatusOK, h.cache.Stats())
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateLinkRequest(w, r)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateURL(req.URL); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var code string
	for attempt := 0; attempt < createLinkMaxAttempts; attempt++ {
		var err error
		code, err = h.shortener.Next()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		err = h.store.Save(r.Context(), code, req.URL)
		if err == nil {
			shortURL := h.shortURL(code)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Location", shortURL)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createLinkResponse{
				Code:     code,
				ShortURL: shortURL,
			})
			return
		}
		if !errors.Is(err, storage.ErrConflict) {
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// 7 文字 base62 の空間は 62^7、約 3.5 兆通りあります。
		// 衝突確率は極小ですが 0 ではないため、保存時の ErrConflict だけを小さく再試行します。
	}

	writeJSONError(w, http.StatusInternalServerError, "internal server error")
}

func decodeCreateLinkRequest(w http.ResponseWriter, r *http.Request) (createLinkRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, createLinkMaxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req createLinkRequest
	if err := decoder.Decode(&req); err != nil {
		return createLinkRequest{}, err
	}

	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return createLinkRequest{}, errors.New("invalid trailing json")
	}

	return req, nil
}

func (h *Handler) getLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	link, err := h.store.Get(r.Context(), code)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, linkResponse{
		Code:     link.Code,
		URL:      link.URL,
		ShortURL: h.shortURL(link.Code),
	})
}

func (h *Handler) getLinkStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	link, err := h.store.Get(r.Context(), code)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, linkStatsResponse{
		Code:   link.Code,
		URL:    link.URL,
		Clicks: link.Clicks,
	})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	originalURL, err := h.store.Load(r.Context(), code)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	if h.clicks != nil {
		h.clicks.Record(code)
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

func writeStorageError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "link not found")
		return
	}

	writeJSONError(w, http.StatusInternalServerError, "internal server error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
