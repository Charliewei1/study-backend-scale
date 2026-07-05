// Package shortener は、内部の連番 ID を短い文字列へ変換します。
package shortener

import "sync"

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Shortener は次に使う ID を覚えておく小さな採番器です。
//
// net/http はリクエストごとに goroutine を使うため、複数リクエストが同時に
// Next を呼んでも ID が重ならないように mutex で守ります。
type Shortener struct {
	mu     sync.Mutex
	nextID uint64
}

// New は 1 から採番を始める Shortener を返します。
// 0 も base62 では表せますが、最初のコードを "1" にすると教材として見やすくなります。
func New() *Shortener {
	return &Shortener{nextID: 1}
}

// Next は新しい連番 ID を取り出し、base62 文字列へ変換します。
func (s *Shortener) Next() string {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.mu.Unlock()

	return Encode(id)
}

// Encode は 10 進数の ID を 62 進数の文字列に変換します。
//
// 使う文字は 0-9, a-z, A-Z の 62 種類です。10 進数を文字列に直す時と同じで、
// 62 で割った余りを 1 桁として取り出し、商が 0 になるまで繰り返します。
func Encode(id uint64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	var reversed []byte
	base := uint64(len(alphabet))
	for id > 0 {
		remainder := id % base
		reversed = append(reversed, alphabet[remainder])
		id = id / base
	}

	// 余りは下の桁から出てくるため、最後に反転して人が読む順番に戻します。
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return string(reversed)
}
