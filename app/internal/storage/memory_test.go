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

func TestMemoryStoreOverwrite(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(context.Background(), "1", "https://old.example.com"); err != nil {
		t.Fatalf("Save old URL returned error: %v", err)
	}
	if err := store.Save(context.Background(), "1", "https://new.example.com"); err != nil {
		t.Fatalf("Save new URL returned error: %v", err)
	}

	got, err := store.Load(context.Background(), "1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://new.example.com" {
		t.Fatalf("Load returned %q, want overwritten URL", got)
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
	if got != (Link{Code: "abc", URL: "https://example.com/articles/1"}) {
		t.Fatalf("Get = %#v, want saved link", got)
	}
}
