package storage

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPostgresStoreSaveLoadAndGet(t *testing.T) {
	store, prefix := newTestPostgresStore(t)
	ctx := context.Background()
	code := prefix + "_abc"

	if err := store.Save(ctx, code, "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	gotURL, err := store.Load(ctx, code)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if gotURL != "https://example.com/articles/1" {
		t.Fatalf("Load returned %q, want saved URL", gotURL)
	}

	gotLink, err := store.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if gotLink != (Link{Code: code, URL: "https://example.com/articles/1", Clicks: 0}) {
		t.Fatalf("Get = %#v, want saved link", gotLink)
	}
}

func TestPostgresStoreLoadMissing(t *testing.T) {
	store, prefix := newTestPostgresStore(t)

	if got, err := store.Load(context.Background(), prefix+"_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load = %q, %v; want ErrNotFound", got, err)
	}
}

func TestPostgresStoreSaveConflict(t *testing.T) {
	store, prefix := newTestPostgresStore(t)
	ctx := context.Background()
	code := prefix + "_abc"

	if err := store.Save(ctx, code, "https://old.example.com"); err != nil {
		t.Fatalf("Save old URL returned error: %v", err)
	}
	if err := store.Save(ctx, code, "https://new.example.com"); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save duplicate URL returned %v, want ErrConflict", err)
	}

	got, err := store.Load(ctx, code)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://old.example.com" {
		t.Fatalf("Load returned %q, want original URL", got)
	}
}

func TestPostgresStoreIncrementClicks(t *testing.T) {
	store, prefix := newTestPostgresStore(t)
	ctx := context.Background()
	code := prefix + "_abc"

	if err := store.Save(ctx, code, "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := store.IncrementClicks(ctx, code); err != nil {
			t.Fatalf("IncrementClicks returned error: %v", err)
		}
	}

	got, err := store.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Clicks != 3 {
		t.Fatalf("Clicks = %d, want 3", got.Clicks)
	}
}

func TestPostgresStoreIncrementClicksMissing(t *testing.T) {
	store, prefix := newTestPostgresStore(t)

	if err := store.IncrementClicks(context.Background(), prefix+"_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("IncrementClicks returned %v, want ErrNotFound", err)
	}
}

func newTestPostgresStore(t *testing.T) (*PostgresStore, string) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	store, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresStore returned error: %v", err)
	}

	prefix := "test_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() {
		if _, err := store.pool.Exec(context.Background(), `DELETE FROM links WHERE code LIKE $1`, prefix+"%"); err != nil {
			t.Fatalf("cleanup postgres rows: %v", err)
		}
		store.Close()
	})
	return store, prefix
}
