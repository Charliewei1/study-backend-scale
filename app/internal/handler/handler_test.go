package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/study-backend-scale/shortlink/internal/shortener"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const testBaseURL = "http://example.test"

func newTestHandler() (http.Handler, *storage.MemoryStore) {
	store := storage.NewMemoryStore()
	return New(shortener.New(), store, testBaseURL), store
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
			store.Save(tt.seedCode, tt.seedURL)
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
			store.Save(tt.seedCode, tt.seedURL)
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
