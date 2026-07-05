// Package storage は短縮コードと元 URL の対応を保存します。
package storage

import (
	"context"
	"errors"
)

// ErrNotFound は指定された短縮コードが保存先に存在しないことを表します。
var ErrNotFound = errors.New("link not found")

// Link は保存済みリンクのメタ情報です。
type Link struct {
	Code string
	URL  string
}

// Storage は保存先を差し替えるためのインターフェースです。
//
// context.Context を受け取ることで、HTTP リクエストのキャンセルや将来の
// タイムアウト設定を storage 層まで伝播できます。
type Storage interface {
	Save(ctx context.Context, code, url string) error
	Load(ctx context.Context, code string) (string, error)
	Get(ctx context.Context, code string) (Link, error)
}
