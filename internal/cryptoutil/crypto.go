// Package cryptoutil loads RSA keys and encrypts request bodies.
package cryptoutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

const (
	// Header marks request bodies encrypted by the agent.
	Header = "X-Encrypted-Body"
	// Algorithm identifies the request body encryption algorithm.
	Algorithm = "RSA-OAEP-SHA256"
)

// LoadPublicKey reads an RSA public key from a PEM file.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("decode public key PEM")
	}

	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not RSA")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	if certificate, err := x509.ParseCertificate(block.Bytes); err == nil {
		key, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not RSA")
		}
		return key, nil
	}

	return nil, errors.New("parse RSA public key")
}

// LoadPrivateKey reads an RSA private key from a PEM file.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("decode private key PEM")
	}

	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		if err := key.Validate(); err != nil {
			return nil, fmt.Errorf("validate private key: %w", err)
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if err := key.Validate(); err != nil {
			return nil, fmt.Errorf("validate private key: %w", err)
		}
		return key, nil
	}

	return nil, errors.New("parse RSA private key")
}

// Encrypt encrypts data with RSA-OAEP in blocks.
func Encrypt(data []byte, key *rsa.PublicKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New("public key is nil")
	}
	blockSize := key.Size() - 2*sha256.Size - 2
	if blockSize <= 0 {
		return nil, errors.New("RSA key is too small")
	}
	if len(data) == 0 {
		return []byte{}, nil
	}

	blockCount := (len(data) + blockSize - 1) / blockSize
	encrypted := make([]byte, 0, blockCount*key.Size())
	for offset := 0; offset < len(data); offset += blockSize {
		end := min(offset+blockSize, len(data))
		block, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, key, data[offset:end], nil)
		if err != nil {
			return nil, fmt.Errorf("encrypt block: %w", err)
		}
		encrypted = append(encrypted, block...)
	}
	return encrypted, nil
}

// Decrypt decrypts RSA-OAEP blocks produced by Encrypt.
func Decrypt(data []byte, key *rsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New("private key is nil")
	}
	blockSize := key.PublicKey.Size()
	if blockSize == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid encrypted body size")
	}
	if len(data) == 0 {
		return []byte{}, nil
	}

	plainBlockSize := blockSize - 2*sha256.Size - 2
	decrypted := make([]byte, 0, len(data)/blockSize*plainBlockSize)
	for offset := 0; offset < len(data); offset += blockSize {
		block, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, data[offset:offset+blockSize], nil)
		if err != nil {
			return nil, fmt.Errorf("decrypt block: %w", err)
		}
		decrypted = append(decrypted, block...)
	}
	return decrypted, nil
}
