package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore は PostgreSQL を使う永続化ストアです。
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore は PostgreSQL 接続プールを作り、必要なテーブルを初期化します。
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	store := &PostgresStore{pool: pool}
	if err := store.init(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) init(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS links (
	code TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	clicks BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("initialize postgres schema: %w", err)
	}
	return nil
}

// Close は PostgreSQL 接続プールを閉じます。
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// Save は code と url の対応を PostgreSQL に保存します。
func (s *PostgresStore) Save(ctx context.Context, code, url string) error {
	const query = `
INSERT INTO links (code, url)
VALUES ($1, $2)
ON CONFLICT (code) DO UPDATE SET url = excluded.url;`

	if _, err := s.pool.Exec(ctx, query, code, url); err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

// Load は code に対応する URL を返します。
func (s *PostgresStore) Load(ctx context.Context, code string) (string, error) {
	link, err := s.Get(ctx, code)
	if err != nil {
		return "", err
	}
	return link.URL, nil
}

// Get は code に対応するリンクのメタ情報を返します。
func (s *PostgresStore) Get(ctx context.Context, code string) (Link, error) {
	const query = `SELECT code, url, clicks FROM links WHERE code = $1;`

	var link Link
	if err := s.pool.QueryRow(ctx, query, code).Scan(&link.Code, &link.URL, &link.Clicks); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Link{}, ErrNotFound
		}
		return Link{}, fmt.Errorf("get link: %w", err)
	}
	return link, nil
}

// IncrementClicks は code に対応するリンクのクリック数を 1 増やします。
func (s *PostgresStore) IncrementClicks(ctx context.Context, code string) error {
	const query = `UPDATE links SET clicks = clicks + 1 WHERE code = $1;`

	tag, err := s.pool.Exec(ctx, query, code)
	if err != nil {
		return fmt.Errorf("increment clicks: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
