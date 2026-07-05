package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

func TestRedisStoreLoadMissFallsBackAndCaches(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore(t)
	if err := store.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	client := newFakeRedisClient()
	cached := newRedisStore(store, client, time.Hour)

	got, err := cached.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://example.com/articles/1" {
		t.Fatalf("Load = %q, want original URL", got)
	}
	if store.loads != 1 {
		t.Fatalf("storage loads = %d, want 1", store.loads)
	}
	if client.setCalls != 1 {
		t.Fatalf("redis set calls = %d, want 1", client.setCalls)
	}
	if client.lastSetKey != "shortlink:url:abc" {
		t.Fatalf("redis set key = %q, want shortlink:url:abc", client.lastSetKey)
	}
	if client.lastSetTTL != time.Hour {
		t.Fatalf("redis set ttl = %s, want 1h", client.lastSetTTL)
	}

	got, err = cached.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("second Load returned error: %v", err)
	}
	if got != "https://example.com/articles/1" {
		t.Fatalf("second Load = %q, want original URL", got)
	}
	if store.loads != 1 {
		t.Fatalf("storage loads after hit = %d, want 1", store.loads)
	}

	stats := cached.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("stats = %+v, want hits=1 misses=1", stats)
	}
}

func TestRedisStoreLoadFallsBackWhenRedisGetFails(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore(t)
	if err := store.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	client := newFakeRedisClient()
	client.getErr = errors.New("redis unavailable")
	client.setErr = errors.New("redis unavailable")
	cached := newRedisStore(store, client, time.Hour)

	got, err := cached.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://example.com/articles/1" {
		t.Fatalf("Load = %q, want original URL", got)
	}
	if store.loads != 1 {
		t.Fatalf("storage loads = %d, want 1", store.loads)
	}

	stats := cached.Stats()
	if stats.Hits != 0 || stats.Misses != 1 {
		t.Fatalf("stats = %+v, want hits=0 misses=1", stats)
	}
}

func TestRedisStoreLoadNotFoundDoesNotCache(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore(t)
	client := newFakeRedisClient()
	cached := newRedisStore(store, client, time.Hour)

	_, err := cached.Load(ctx, "missing")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Load error = %v, want ErrNotFound", err)
	}
	if client.setCalls != 0 {
		t.Fatalf("redis set calls = %d, want 0", client.setCalls)
	}
}

func TestRedisStoreDelegatesNonLoadOperations(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore(t)
	cached := newRedisStore(store, newFakeRedisClient(), time.Hour)

	if err := cached.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	link, err := cached.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if link.URL != "https://example.com/articles/1" {
		t.Fatalf("Get URL = %q, want original URL", link.URL)
	}
	if err := cached.IncrementClicks(ctx, "abc"); err != nil {
		t.Fatalf("IncrementClicks returned error: %v", err)
	}
	link, err = store.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("store Get returned error: %v", err)
	}
	if link.Clicks != 1 {
		t.Fatalf("clicks = %d, want 1", link.Clicks)
	}
}

func TestRedisStoreSavePropagatesConflict(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore(t)
	cached := newRedisStore(store, newFakeRedisClient(), time.Hour)

	if err := cached.Save(ctx, "abc", "https://old.example.com"); err != nil {
		t.Fatalf("initial Save returned error: %v", err)
	}
	if err := cached.Save(ctx, "abc", "https://new.example.com"); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("duplicate Save returned %v, want ErrConflict", err)
	}

	got, err := store.Load(ctx, "abc")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got != "https://old.example.com" {
		t.Fatalf("Load = %q, want original URL", got)
	}
}

type countingStore struct {
	*storage.MemoryStore
	loads int
}

func newCountingStore(t *testing.T) *countingStore {
	t.Helper()
	return &countingStore{MemoryStore: storage.NewMemoryStore()}
}

func (s *countingStore) Load(ctx context.Context, code string) (string, error) {
	s.loads++
	return s.MemoryStore.Load(ctx, code)
}

type fakeRedisClient struct {
	values map[string]string

	getErr error
	setErr error

	setCalls   int
	lastSetKey string
	lastSetTTL time.Duration
}

func newFakeRedisClient() *fakeRedisClient {
	return &fakeRedisClient{
		values: make(map[string]string),
	}
}

func (c *fakeRedisClient) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c.getErr != nil {
		return "", c.getErr
	}
	value, ok := c.values[key]
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func (c *fakeRedisClient) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.setCalls++
	c.lastSetKey = key
	c.lastSetTTL = ttl
	if c.setErr != nil {
		return c.setErr
	}
	c.values[key] = value
	return nil
}
