package storage

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreSaveAndLoad(t *testing.T) {
	tests := []struct {
		name string
		code string
		url  string
	}{
		{name: "short code", code: "1", url: "https://example.com"},
		{name: "base62 code", code: "abcZ9", url: "https://example.com/articles/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			if err := store.Save(context.Background(), tt.code, tt.url); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}

			got, err := store.Load(context.Background(), tt.code)
			if err != nil {
				t.Fatalf("Load(%q) returned error: %v", tt.code, err)
			}
			if got != tt.url {
				t.Fatalf("Load(%q) url = %q, want %q", tt.code, got, tt.url)
			}
		})
	}
}

func TestMemoryStoreLoadMissing(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "empty store", code: "missing"},
		{name: "different code", code: "2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			if err := store.Save(context.Background(), "1", "https://example.com"); err != nil {
				t.Fatalf("Save returned error: %v", err)
			}

			if got, err := store.Load(context.Background(), tt.code); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Load(%q) = %q, %v; want ErrNotFound", tt.code, got, err)
			}
		})
	}
}

func TestMemoryStoreSaveConflict(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(context.Background(), "1", "https://old.example.com"); err != nil {
		t.Fatalf("Save old URL returned error: %v", err)
	}
	if err := store.Save(context.Background(), "1", "https://new.example.com"); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save duplicate URL returned %v, want ErrConflict", err)
	}

	got, err := store.Load(context.Background(), "1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://old.example.com" {
		t.Fatalf("Load returned %q, want original URL", got)
	}
}

func TestMemoryStoreGet(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(context.Background(), "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Get(context.Background(), "abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != (Link{Code: "abc", URL: "https://example.com/articles/1", Clicks: 0}) {
		t.Fatalf("Get = %#v, want saved link", got)
	}
}

func TestMemoryStoreIncrementClicks(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	if err := store.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := store.IncrementClicks(ctx, "abc"); err != nil {
			t.Fatalf("IncrementClicks returned error: %v", err)
		}
	}

	got, err := store.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Clicks != 3 {
		t.Fatalf("Clicks = %d, want 3", got.Clicks)
	}
}

func TestMemoryStoreIncrementClicksMissing(t *testing.T) {
	store := NewMemoryStore()

	if err := store.IncrementClicks(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("IncrementClicks returned %v, want ErrNotFound", err)
	}
}
