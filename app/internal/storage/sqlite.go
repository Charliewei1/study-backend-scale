package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteStore は SQLite ファイルを使う永続化ストアです。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore は SQLite 接続を開き、必要なテーブルを初期化します。
func NewSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS links (
	code TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize sqlite schema: %w", err)
	}
	return nil
}

// Close は SQLite 接続プールを閉じます。
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Save は code と url の対応を SQLite に保存します。
func (s *SQLiteStore) Save(ctx context.Context, code, url string) error {
	const query = `
INSERT INTO links (code, url)
VALUES (?, ?)
ON CONFLICT(code) DO UPDATE SET url = excluded.url;`

	if _, err := s.db.ExecContext(ctx, query, code, url); err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	return nil
}

// Load は code に対応する URL を返します。
func (s *SQLiteStore) Load(ctx context.Context, code string) (string, error) {
	link, err := s.Get(ctx, code)
	if err != nil {
		return "", err
	}
	return link.URL, nil
}

// Get は code に対応するリンクのメタ情報を返します。
func (s *SQLiteStore) Get(ctx context.Context, code string) (Link, error) {
	const query = `SELECT code, url FROM links WHERE code = ?;`

	var link Link
	if err := s.db.QueryRowContext(ctx, query, code).Scan(&link.Code, &link.URL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Link{}, ErrNotFound
		}
		return Link{}, fmt.Errorf("get link: %w", err)
	}
	return link, nil
}
