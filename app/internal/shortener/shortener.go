// Package shortener は、URL 短縮用のランダムな短いコードを生成します。
package shortener

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const codeLength = 7

var alphabetSize = big.NewInt(int64(len(alphabet)))

// Shortener はランダムな base62 コードを作る採番器です。
//
// 連番カウンタは 1 プロセス内では簡単ですが、複数レプリカでは各 Pod が同じ
// 1, 2, 3... を発行して衝突します。Pod 再起動でもカウンタが戻るため、
// ステートレスな水平スケールには向きません。
type Shortener struct{}

// New はランダムコードを生成する Shortener を返します。
func New() *Shortener {
	return &Shortener{}
}

// Next は 7 文字の base62 コードを crypto/rand で生成します。
func (s *Shortener) Next() (string, error) {
	code := make([]byte, codeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("generate random code: %w", err)
		}
		code[i] = alphabet[n.Int64()]
	}

	return string(code), nil
}
