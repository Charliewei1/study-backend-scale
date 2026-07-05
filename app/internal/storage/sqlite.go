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
	clicks INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize sqlite schema: %w", err)
	}
	if err := s.ensureClicksColumn(ctx); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureClicksColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(links);`)
	if err != nil {
		return fmt.Errorf("inspect sqlite schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan sqlite schema: %w", err)
		}
		if name == "clicks" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite schema: %w", err)
	}

	const alter = `ALTER TABLE links ADD COLUMN clicks INTEGER NOT NULL DEFAULT 0;`
	if _, err := s.db.ExecContext(ctx, alter); err != nil {
		return fmt.Errorf("add clicks column: %w", err)
	}
	return nil
}

// Close は SQLite 接続プールを閉じます。
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Ping は SQLite 接続が利用可能かを確認します。
func (s *SQLiteStore) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	return nil
}

// Save は code と url の対応を SQLite に保存します。
func (s *SQLiteStore) Save(ctx context.Context, code, url string) error {
	const query = `
INSERT INTO links (code, url)
VALUES (?, ?)
ON CONFLICT(code) DO NOTHING;`

	result, err := s.db.ExecContext(ctx, query, code, url)
	if err != nil {
		return fmt.Errorf("save link: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read save result: %w", err)
	}
	if affected == 0 {
		return ErrConflict
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
	const query = `SELECT code, url, clicks FROM links WHERE code = ?;`

	var link Link
	if err := s.db.QueryRowContext(ctx, query, code).Scan(&link.Code, &link.URL, &link.Clicks); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Link{}, ErrNotFound
		}
		return Link{}, fmt.Errorf("get link: %w", err)
	}
	return link, nil
}

// IncrementClicks は code に対応するリンクのクリック数を 1 増やします。
func (s *SQLiteStore) IncrementClicks(ctx context.Context, code string) error {
	const query = `UPDATE links SET clicks = clicks + 1 WHERE code = ?;`

	result, err := s.db.ExecContext(ctx, query, code)
	if err != nil {
		return fmt.Errorf("increment clicks: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read increment result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
