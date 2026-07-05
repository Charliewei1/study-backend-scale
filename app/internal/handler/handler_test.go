package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/study-backend-scale/shortlink/internal/analytics"
	"github.com/study-backend-scale/shortlink/internal/cache"
	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const testBaseURL = "http://example.test"

func newTestHandler() (http.Handler, *storage.MemoryStore) {
	store := storage.NewMemoryStore()
	return New(shortener.New(), store, testBaseURL, nil), store
}

func TestCreateLink(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantStatus   int
		wantLocation string
		wantBody     map[string]string
	}{
		{
			name:         "valid https url",
			body:         `{"url":"https://example.com/articles/1"}`,
			wantStatus:   http.StatusCreated,
			wantLocation: testBaseURL + "/1",
			wantBody: map[string]string{
				"code":      "1",
				"short_url": testBaseURL + "/1",
			},
		},
		{
			name:         "valid http url",
			body:         `{"url":"http://example.com"}`,
			wantStatus:   http.StatusCreated,
			wantLocation: testBaseURL + "/1",
			wantBody: map[string]string{
				"code":      "1",
				"short_url": testBaseURL + "/1",
			},
		},
		{
			name:       "invalid json",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "invalid json",
			},
		},
		{
			name:       "empty url",
			body:       `{"url":""}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url is required",
			},
		},
		{
			name:       "blank url",
			body:       `{"url":"   "}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url is required",
			},
		},
		{
			name:       "unsupported scheme",
			body:       `{"url":"ftp://example.com/file"}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url must use http or https",
			},
		},
		{
			name:       "missing scheme",
			body:       `{"url":"example.com"}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url must use http or https",
			},
		},
		{
			name:       "missing host",
			body:       `{"url":"https:"}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url host is required",
			},
		},
		{
			name:       "parse error",
			body:       `{"url":"http://[::1"}`,
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]string{
				"error": "url is invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler()
			req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q, want %q", got, tt.wantLocation)
			}
			assertJSONBody(t, rec.Body.String(), tt.wantBody)
		})
	}
}

func TestCreateLinkRetriesAfterCodeConflict(t *testing.T) {
	store := &conflictOnceStore{
		MemoryStore:        storage.NewMemoryStore(),
		conflictsRemaining: 1,
	}
	h := New(shortener.New(), store, testBaseURL, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://example.com/articles/1"}`))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != testBaseURL+"/2" {
		t.Fatalf("Location = %q, want %q", got, testBaseURL+"/2")
	}
	assertJSONBody(t, rec.Body.String(), map[string]string{
		"code":      "2",
		"short_url": testBaseURL + "/2",
	})
	if got, want := strings.Join(store.savedCodes, ","), "1,2"; got != want {
		t.Fatalf("saved codes = %q, want %q", got, want)
	}

	gotURL, err := store.Load(context.Background(), "2")
	if err != nil {
		t.Fatalf("Load retried code returned error: %v", err)
	}
	if gotURL != "https://example.com/articles/1" {
		t.Fatalf("Load retried code = %q, want original URL", gotURL)
	}
}

func TestGetLink(t *testing.T) {
	tests := []struct {
		name       string
		seedCode   string
		seedURL    string
		path       string
		wantStatus int
		wantBody   map[string]string
	}{
		{
			name:       "found",
			seedCode:   "abc",
			seedURL:    "https://example.com/articles/1",
			path:       "/api/links/abc",
			wantStatus: http.StatusOK,
			wantBody: map[string]string{
				"code":      "abc",
				"url":       "https://example.com/articles/1",
				"short_url": testBaseURL + "/abc",
			},
		},
		{
			name:       "not found",
			seedCode:   "abc",
			seedURL:    "https://example.com/articles/1",
			path:       "/api/links/missing",
			wantStatus: http.StatusNotFound,
			wantBody: map[string]string{
				"error": "link not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, store := newTestHandler()
			if err := store.Save(context.Background(), tt.seedCode, tt.seedURL); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			assertJSONBody(t, rec.Body.String(), tt.wantBody)
		})
	}
}

func TestRedirect(t *testing.T) {
	tests := []struct {
		name         string
		seedCode     string
		seedURL      string
		path         string
		wantStatus   int
		wantLocation string
		wantBody     map[string]string
	}{
		{
			name:         "found",
			seedCode:     "abc",
			seedURL:      "https://example.com/articles/1",
			path:         "/abc",
			wantStatus:   http.StatusFound,
			wantLocation: "https://example.com/articles/1",
		},
		{
			name:       "not found",
			seedCode:   "abc",
			seedURL:    "https://example.com/articles/1",
			path:       "/missing",
			wantStatus: http.StatusNotFound,
			wantBody: map[string]string{
				"error": "link not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, store := newTestHandler()
			if err := store.Save(context.Background(), tt.seedCode, tt.seedURL); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q, want %q", got, tt.wantLocation)
			}
			if tt.wantBody != nil {
				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Fatalf("Content-Type = %q, want application/json", got)
				}
				assertJSONBody(t, rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestLinkStatsAfterRedirect(t *testing.T) {
	store := storage.NewMemoryStore()
	clicks := analytics.New(store, 8)
	h := New(shortener.New(), store, testBaseURL, clicks)

	if err := store.Save(context.Background(), "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	redirectReq := httptest.NewRequest(http.MethodGet, "/abc", nil)
	redirectRec := httptest.NewRecorder()
	h.ServeHTTP(redirectRec, redirectReq)
	if redirectRec.Code != http.StatusFound {
		t.Fatalf("redirect status = %d, want %d; body=%s", redirectRec.Code, http.StatusFound, redirectRec.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := clicks.Close(ctx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/api/links/abc/stats", nil)
	statsRec := httptest.NewRecorder()
	h.ServeHTTP(statsRec, statsReq)

	if statsRec.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want %d; body=%s", statsRec.Code, http.StatusOK, statsRec.Body.String())
	}
	if got := statsRec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var got linkStatsResponse
	if err := json.Unmarshal(statsRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, statsRec.Body.String())
	}
	want := linkStatsResponse{
		Code:   "abc",
		URL:    "https://example.com/articles/1",
		Clicks: 1,
	}
	if got != want {
		t.Fatalf("stats = %#v, want %#v", got, want)
	}
}

func TestHealthz(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name       string
		store      storage.Storage
		wantStatus int
		wantBody   string
	}{
		{
			name:       "storage available",
			store:      storage.NewMemoryStore(),
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "storage unavailable",
			store:      failingPingStore{MemoryStore: storage.NewMemoryStore()},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "storage unavailable\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(shortener.New(), tt.store, testBaseURL, nil)
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Fatalf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestCacheStats(t *testing.T) {
	store := storage.NewMemoryStore()
	h := New(shortener.New(), store, testBaseURL, nil, fakeCacheStats{
		stats: cache.Stats{
			Hits:   7,
			Misses: 3,
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/cache/stats", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var got cache.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, rec.Body.String())
	}
	if got.Hits != 7 || got.Misses != 3 {
		t.Fatalf("cache stats = %+v, want hits=7 misses=3", got)
	}
}

type fakeCacheStats struct {
	stats cache.Stats
}

func (s fakeCacheStats) Stats() cache.Stats {
	return s.stats
}

type failingPingStore struct {
	*storage.MemoryStore
}

func (s failingPingStore) Ping(ctx context.Context) error {
	return errors.New("storage down")
}

type conflictOnceStore struct {
	*storage.MemoryStore
	conflictsRemaining int
	savedCodes         []string
}

func (s *conflictOnceStore) Save(ctx context.Context, code, url string) error {
	s.savedCodes = append(s.savedCodes, code)
	if s.conflictsRemaining > 0 {
		s.conflictsRemaining--
		return storage.ErrConflict
	}
	return s.MemoryStore.Save(ctx, code, url)
}

func assertJSONBody(t *testing.T, body string, want map[string]string) {
	t.Helper()

	var got map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("response body is not JSON: %v; body=%s", err, body)
	}

	if len(got) != len(want) {
		t.Fatalf("body field count = %d, want %d; body=%v", len(got), len(want), got)
	}
	for key, wantValue := range want {
		if gotValue := got[key]; gotValue != wantValue {
			t.Fatalf("body[%q] = %q, want %q; body=%v", key, gotValue, wantValue, got)
		}
	}
}
