package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSQLiteStoreSaveLoadAndGet(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	gotURL, err := store.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if gotURL != "https://example.com/articles/1" {
		t.Fatalf("Load returned %q, want saved URL", gotURL)
	}

	gotLink, err := store.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if gotLink != (Link{Code: "abc", URL: "https://example.com/articles/1", Clicks: 0}) {
		t.Fatalf("Get = %#v, want saved link", gotLink)
	}
}

func TestSQLiteStoreLoadMissing(t *testing.T) {
	store := newTestSQLiteStore(t)

	if got, err := store.Load(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load = %q, %v; want ErrNotFound", got, err)
	}
}

func TestSQLiteStoreOverwrite(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "abc", "https://old.example.com"); err != nil {
		t.Fatalf("Save old URL returned error: %v", err)
	}
	if err := store.Save(ctx, "abc", "https://new.example.com"); err != nil {
		t.Fatalf("Save new URL returned error: %v", err)
	}

	got, err := store.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://new.example.com" {
		t.Fatalf("Load returned %q, want overwritten URL", got)
	}
}

func TestSQLiteStoreIncrementClicks(t *testing.T) {
	store := newTestSQLiteStore(t)
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

func TestSQLiteStoreIncrementClicksMissing(t *testing.T) {
	store := newTestSQLiteStore(t)

	if err := store.IncrementClicks(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("IncrementClicks returned %v, want ErrNotFound", err)
	}
}

func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(context.Background(), path)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	return store
}
