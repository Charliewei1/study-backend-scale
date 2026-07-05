package analytics

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/study-backend-scale/shortlink/internal/storage"
)

func TestCollectorAggregatesClicks(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := context.Background()
	if err := store.Save(ctx, "abc", "https://example.com/articles/1"); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	const total = 1000
	collector := New(store, total)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok := collector.Record("abc"); !ok {
				t.Errorf("Record returned false")
			}
		}()
	}
	wg.Wait()

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := collector.Close(closeCtx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	got, err := store.Get(ctx, "abc")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Clicks != total {
		t.Fatalf("Clicks = %d, want %d", got.Clicks, total)
	}
	if collector.Dropped() != 0 {
		t.Fatalf("Dropped = %d, want 0", collector.Dropped())
	}
	if collector.Failed() != 0 {
		t.Fatalf("Failed = %d, want 0", collector.Failed())
	}
}

func TestCollectorDropsWhenBufferIsFull(t *testing.T) {
	store := newBlockingStore()
	collector := New(store, 1)

	if ok := collector.Record("abc"); !ok {
		t.Fatalf("first Record returned false")
	}
	<-store.started
	if ok := collector.Record("abc"); !ok {
		t.Fatalf("second Record returned false")
	}
	if ok := collector.Record("abc"); ok {
		t.Fatalf("third Record returned true, want drop")
	}
	if collector.Dropped() != 1 {
		t.Fatalf("Dropped = %d, want 1", collector.Dropped())
	}

	close(store.release)
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := collector.Close(closeCtx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if got := store.count.Load(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

type blockingStore struct {
	startOnce sync.Once
	started   chan struct{}
	release   chan struct{}
	count     atomic.Int64
}

func newBlockingStore() *blockingStore {
	return &blockingStore{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingStore) Save(context.Context, string, string) error {
	return nil
}

func (s *blockingStore) Load(context.Context, string) (string, error) {
	return "", storage.ErrNotFound
}

func (s *blockingStore) Get(context.Context, string) (storage.Link, error) {
	return storage.Link{}, storage.ErrNotFound
}

func (s *blockingStore) IncrementClicks(context.Context, string) error {
	s.startOnce.Do(func() {
		close(s.started)
	})
	<-s.release
	s.count.Add(1)
	return nil
}
