// Package cache provides optional read-through caching for storage.
package cache

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/study-backend-scale/shortlink/internal/metrics"
	"github.com/study-backend-scale/shortlink/internal/storage"
)

const (
	defaultTTL = 24 * time.Hour
	keyPrefix  = "shortlink:url:"
)

type redisClient interface {
	Get(ctx context.Context, key string) (string, error)
	SetEX(ctx context.Context, key, value string, ttl time.Duration) error
}

type goRedisClient struct {
	client *redis.Client
}

func (c goRedisClient) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c goRedisClient) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Stats is a snapshot of cache behavior.
type Stats struct {
	Hits   uint64 `json:"hits"`
	Misses uint64 `json:"misses"`
}

// StatsProvider exposes cache counters without coupling callers to RedisStore.
type StatsProvider interface {
	Stats() Stats
}

// RedisStore wraps a storage.Storage with Redis cache-aside reads for Load.
type RedisStore struct {
	next   storage.Storage
	client redisClient
	ttl    time.Duration

	hits   atomic.Uint64
	misses atomic.Uint64
}

// NewRedisStore creates a cache decorator using github.com/redis/go-redis/v9.
func NewRedisStore(next storage.Storage, client *redis.Client) *RedisStore {
	return newRedisStore(next, goRedisClient{client: client}, defaultTTL)
}

func newRedisStore(next storage.Storage, client redisClient, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &RedisStore{
		next:   next,
		client: client,
		ttl:    ttl,
	}
}

// Save delegates writes to the wrapped storage. New codes are unique in this
// app, so redirect reads populate Redis on first Load miss.
func (s *RedisStore) Save(ctx context.Context, code, url string) error {
	return s.next.Save(ctx, code, url)
}

// Ping delegates readiness checks to the authoritative storage.
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.next.Ping(ctx)
}

// Load returns code's URL from Redis when possible, falling back to storage on
// misses or Redis failures. Redis errors are logged but never returned.
func (s *RedisStore) Load(ctx context.Context, code string) (string, error) {
	key := cacheKey(code)
	if url, err := s.client.Get(ctx, key); err == nil {
		s.hits.Add(1)
		metrics.RecordCacheHit()
		return url, nil
	} else {
		s.misses.Add(1)
		metrics.RecordCacheMiss()
		if !errors.Is(err, redis.Nil) {
			slog.Warn("redis get failed; falling back to storage", "code", code, "error", err)
		}
	}

	url, err := s.next.Load(ctx, code)
	if err != nil {
		return "", err
	}

	if err := s.client.SetEX(ctx, key, url, s.ttl); err != nil {
		slog.Warn("redis set failed", "code", code, "error", err)
	}
	return url, nil
}

// Get delegates metadata reads to storage so click counts stay authoritative.
func (s *RedisStore) Get(ctx context.Context, code string) (storage.Link, error) {
	return s.next.Get(ctx, code)
}

// IncrementClicks delegates click writes to storage.
func (s *RedisStore) IncrementClicks(ctx context.Context, code string) error {
	return s.next.IncrementClicks(ctx, code)
}

// Stats returns atomic hit/miss counters for lightweight verification.
func (s *RedisStore) Stats() Stats {
	return Stats{
		Hits:   s.hits.Load(),
		Misses: s.misses.Load(),
	}
}

func cacheKey(code string) string {
	return keyPrefix + code
}
