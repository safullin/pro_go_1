package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const Header = "HashSHA256"

// Sum возвращает HMAC-SHA256 для data с симметричным ключом key.
func Sum(data []byte, key string) string {
	hash := hmac.New(sha256.New, []byte(key))
	_, _ = hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// Valid проверяет подпись data для ключа key.
func Valid(data []byte, key string, got string) bool {
	if got == "" {
		return false
	}
	want := Sum(data, key)
	return hmac.Equal([]byte(want), []byte(got))
}
