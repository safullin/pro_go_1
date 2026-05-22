package auth

import "testing"

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !CheckPassword(hash, "secret") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected password mismatch")
	}
}

func TestToken(t *testing.T) {
	token := NewToken(42)
	id, err := UserID(token)
	if err != nil {
		t.Fatalf("user id from token: %v", err)
	}
	if id != 42 {
		t.Fatalf("unexpected id: got %d want %d", id, 42)
	}
	if _, err := UserID(token + "x"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
