package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// EncryptedValuePrefix marks ciphertext stored in user_configs.
// Format: enc:v1:<base64(nonce || ciphertext)>
const EncryptedValuePrefix = "enc:v1:"

var (
	// ErrInvalidKeySize is returned when the master key is not 32 bytes.
	ErrInvalidKeySize = errors.New("config encryption key must be 32 bytes")
)

// DeriveConfigEncryptionKey returns a 32-byte AES key. If `rawKey` is empty,
// it derives one from `fallbackSeed` (typically the JWT_SECRET) via SHA-256.
// Returns (key, derivedFromFallback).
func DeriveConfigEncryptionKey(rawKey, fallbackSeed string) ([]byte, bool) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed != "" {
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
			return decoded, false
		}
		// Treat as raw passphrase: hash to 32 bytes.
		sum := sha256.Sum256([]byte(trimmed))
		return sum[:], false
	}
	sum := sha256.Sum256([]byte("ailocalbase-config-key|" + fallbackSeed))
	return sum[:], true
}

// EncryptString returns ciphertext with the EncryptedValuePrefix.
// Empty inputs pass through unchanged (no point storing encrypted "").
func EncryptString(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", ErrInvalidKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncryptedValuePrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString reverses EncryptString. If the value lacks the prefix it is
// returned unchanged — supports legacy plaintext rows written before encryption
// was introduced. Those rows are upgraded to ciphertext on next Upsert.
func DecryptString(key []byte, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, EncryptedValuePrefix) {
		return value, nil
	}
	if len(key) != 32 {
		return "", ErrInvalidKeySize
	}
	encoded := strings.TrimPrefix(value, EncryptedValuePrefix)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
