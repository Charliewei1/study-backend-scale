package storage

import (
	"context"
	"sync"
)

// MemoryStore はプロセス内メモリだけを使う保存先です。
//
// map はそのままだと同時読み書きに弱いため、読み取りは RLock、書き込みは Lock で
// 保護します。Day 1 では永続化よりも HTTP と並行処理の基本を見せる目的の実装です。
type MemoryStore struct {
	mu    sync.RWMutex
	links map[string]string
}

// NewMemoryStore は空のインメモリストアを作ります。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		links: make(map[string]string),
	}
}

// Save は code と url の対応を保存します。
func (s *MemoryStore) Save(ctx context.Context, code, url string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.links[code] = url
	return nil
}

// Load は code に対応する URL を返します。
// 見つからない場合は ErrNotFound を返します。
func (s *MemoryStore) Load(ctx context.Context, code string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok := s.links[code]
	if !ok {
		return "", ErrNotFound
	}
	return url, nil
}

// Get は code に対応するリンクのメタ情報を返します。
func (s *MemoryStore) Get(ctx context.Context, code string) (Link, error) {
	url, err := s.Load(ctx, code)
	if err != nil {
		return Link{}, err
	}
	return Link{Code: code, URL: url}, nil
}
