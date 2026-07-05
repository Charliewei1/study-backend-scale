package storage

import "testing"

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
			store.Save(tt.code, tt.url)

			got, ok := store.Load(tt.code)
			if !ok {
				t.Fatalf("Load(%q) ok = false, want true", tt.code)
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
			store.Save("1", "https://example.com")

			if got, ok := store.Load(tt.code); ok {
				t.Fatalf("Load(%q) = %q, true; want ok=false", tt.code, got)
			}
		})
	}
}

func TestMemoryStoreOverwrite(t *testing.T) {
	store := NewMemoryStore()
	store.Save("1", "https://old.example.com")
	store.Save("1", "https://new.example.com")

	got, ok := store.Load("1")
	if !ok {
		t.Fatal("Load returned ok=false, want true")
	}
	if got != "https://new.example.com" {
		t.Fatalf("Load returned %q, want overwritten URL", got)
	}
}
