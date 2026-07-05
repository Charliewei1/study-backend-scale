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
	links map[string]Link
}

// NewMemoryStore は空のインメモリストアを作ります。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		links: make(map[string]Link),
	}
}

// Save は code と url の対応を保存します。
func (s *MemoryStore) Save(ctx context.Context, code, url string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	link := s.links[code]
	link.Code = code
	link.URL = url
	s.links[code] = link
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

	link, ok := s.links[code]
	if !ok {
		return "", ErrNotFound
	}
	return link.URL, nil
}

// Get は code に対応するリンクのメタ情報を返します。
func (s *MemoryStore) Get(ctx context.Context, code string) (Link, error) {
	if err := ctx.Err(); err != nil {
		return Link{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return link, nil
}

// IncrementClicks は code に対応するリンクのクリック数を 1 増やします。
func (s *MemoryStore) IncrementClicks(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[code]
	if !ok {
		return ErrNotFound
	}
	link.Clicks++
	s.links[code] = link
	return nil
}
