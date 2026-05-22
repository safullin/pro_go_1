package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const CookieName = "gophermart_session"

var secret = []byte("gophermart-session-key")

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := passwordSum(salt, password)
	return base64.RawURLEncoding.EncodeToString(salt) + "$" + hex.EncodeToString(sum), nil
}

func CheckPassword(hash string, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	got := passwordSum(salt, password)
	return hmac.Equal(want, got)
}

func NewToken(userID int64) string {
	expires := time.Now().Add(24 * time.Hour).Unix()
	payload := fmt.Sprintf("%d:%d", userID, expires)
	signature := sign(payload)
	raw := payload + ":" + signature
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func UserID(token string) (int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid token")
	}
	payload := parts[0] + ":" + parts[1]
	if !hmac.Equal([]byte(sign(payload)), []byte(parts[2])) {
		return 0, errors.New("invalid token")
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	if time.Now().Unix() > expires {
		return 0, errors.New("expired token")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}

func passwordSum(salt []byte, password string) []byte {
	hash := sha256.New()
	_, _ = hash.Write(salt)
	_, _ = hash.Write([]byte(password))
	return hash.Sum(nil)
}

func sign(payload string) string {
	hash := hmac.New(sha256.New, secret)
	_, _ = hash.Write([]byte(payload))
	return hex.EncodeToString(hash.Sum(nil))
}
