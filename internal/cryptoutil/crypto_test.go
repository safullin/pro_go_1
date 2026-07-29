package cryptoutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	privateKey := generatePrivateKey(t)
	data := bytes.Repeat([]byte("metrics batch payload"), 200)

	encrypted, err := Encrypt(data, &privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	if bytes.Equal(encrypted, data) {
		t.Fatal("encrypted data matches plaintext")
	}

	decrypted, err := Decrypt(encrypted, privateKey)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, data)
	}
}

func TestDecryptRejectsInvalidBody(t *testing.T) {
	privateKey := generatePrivateKey(t)

	if _, err := Decrypt([]byte("invalid"), privateKey); err == nil {
		t.Fatal("Decrypt() error = nil, want error")
	}
}

func TestEncryptDecryptEmptyBody(t *testing.T) {
	privateKey := generatePrivateKey(t)

	encrypted, err := Encrypt(nil, &privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	decrypted, err := Decrypt(encrypted, privateKey)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if len(decrypted) != 0 {
		t.Fatalf("Decrypt() length = %d, want 0", len(decrypted))
	}
}

func TestEncryptRejectsInvalidKey(t *testing.T) {
	if _, err := Encrypt([]byte("body"), nil); err == nil {
		t.Fatal("Encrypt() with nil key error = nil, want error")
	}

	smallKey := &rsa.PublicKey{N: big.NewInt(65537), E: 65537}
	if _, err := Encrypt([]byte("body"), smallKey); err == nil {
		t.Fatal("Encrypt() with small key error = nil, want error")
	}
}

func TestDecryptRejectsNilKey(t *testing.T) {
	if _, err := Decrypt([]byte("body"), nil); err == nil {
		t.Fatal("Decrypt() error = nil, want error")
	}
}

func TestLoadKeys(t *testing.T) {
	privateKey := generatePrivateKey(t)
	directory := t.TempDir()
	publicPath := filepath.Join(directory, "public.pem")
	privatePath := filepath.Join(directory, "private.pem")

	publicData, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	writePEM(t, publicPath, "PUBLIC KEY", publicData)
	writePEM(t, privatePath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privateKey))

	publicKey, err := LoadPublicKey(publicPath)
	if err != nil {
		t.Fatalf("LoadPublicKey() error: %v", err)
	}
	loadedPrivateKey, err := LoadPrivateKey(privatePath)
	if err != nil {
		t.Fatalf("LoadPrivateKey() error: %v", err)
	}
	if publicKey.N.Cmp(privateKey.N) != 0 || publicKey.E != privateKey.E {
		t.Fatal("loaded public key does not match")
	}
	if loadedPrivateKey.N.Cmp(privateKey.N) != 0 {
		t.Fatal("loaded private key does not match")
	}
}

func TestLoadPKCS1PublicAndPKCS8PrivateKeys(t *testing.T) {
	privateKey := generatePrivateKey(t)
	directory := t.TempDir()
	publicPath := filepath.Join(directory, "public.pem")
	privatePath := filepath.Join(directory, "private.pem")

	privateData, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	writePEM(t, publicPath, "RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&privateKey.PublicKey))
	writePEM(t, privatePath, "PRIVATE KEY", privateData)

	if _, err := LoadPublicKey(publicPath); err != nil {
		t.Fatalf("LoadPublicKey() error: %v", err)
	}
	if _, err := LoadPrivateKey(privatePath); err != nil {
		t.Fatalf("LoadPrivateKey() error: %v", err)
	}
}

func TestLoadKeysRejectsInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write invalid key: %v", err)
	}

	if _, err := LoadPublicKey(path); err == nil {
		t.Fatal("LoadPublicKey() error = nil, want error")
	}
	if _, err := LoadPrivateKey(path); err == nil {
		t.Fatal("LoadPrivateKey() error = nil, want error")
	}
}

func generatePrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	return key
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PEM file: %v", err)
	}
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		_ = file.Close()
		t.Fatalf("encode PEM: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close PEM file: %v", err)
	}
}
