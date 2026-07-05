// Package storage は短縮コードと元 URL の対応を保存します。
package storage

import "sync"

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
func (s *MemoryStore) Save(code, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.links[code] = url
}

// Load は code に対応する URL を返します。
// 見つからない場合は ok=false になります。
func (s *MemoryStore) Load(code string) (url string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok = s.links[code]
	return url, ok
}
