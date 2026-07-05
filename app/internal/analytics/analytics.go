// Package analytics はリダイレクト時のクリックイベントを非同期に集計します。
package analytics

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/study-backend-scale/shortlink/internal/storage"
)

// Collector はクリックイベントを受け取り、単一 goroutine で Storage へ反映します。
//
// handler が直接 mutex 付きカウンタを更新する実装でも race は防げますが、その場合は
// リクエスト goroutine が書き込み待ちを受けます。ここでは channel を入口にして集計
// goroutine へ責務を寄せ、満杯時はイベントを捨てて dropped だけ数えることで、
// リダイレクト処理をブロックしない方針を明示しています。
type Collector struct {
	store  storage.Storage
	events chan string
	done   chan struct{}

	mu     sync.RWMutex
	closed bool

	dropped atomic.Uint64
	failed  atomic.Uint64
}

// New は buffer 件までクリックイベントをためられる Collector を開始します。
func New(store storage.Storage, buffer int) *Collector {
	if buffer < 0 {
		buffer = 0
	}

	c := &Collector{
		store:  store,
		events: make(chan string, buffer),
		done:   make(chan struct{}),
	}
	go c.run()
	return c
}

// Record は code のクリックイベントを送ります。
// チャネルが満杯、または Collector が Close 済みの場合は false を返します。
func (c *Collector) Record(code string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false
	}

	select {
	case c.events <- code:
		return true
	default:
		c.dropped.Add(1)
		return false
	}
}

// Dropped はチャネル満杯で捨てたイベント数を返します。
func (c *Collector) Dropped() uint64 {
	return c.dropped.Load()
}

// Failed は Storage への反映に失敗したイベント数を返します。
func (c *Collector) Failed() uint64 {
	return c.failed.Load()
}

// Close は入力を閉じ、キュー済みイベントが Storage に反映されるまで待ちます。
func (c *Collector) Close(ctx context.Context) error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.events)
	}
	c.mu.Unlock()

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Collector) run() {
	defer close(c.done)

	for code := range c.events {
		if err := c.store.IncrementClicks(context.Background(), code); err != nil {
			c.failed.Add(1)
		}
	}
}
