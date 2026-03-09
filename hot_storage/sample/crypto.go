package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

var shareEncryptionKey []byte

func initEncryptionKey() error {
	keyHex := os.Getenv("SHARE_ENCRYPTION_KEY")
	if keyHex == "" {
		return fmt.Errorf("SHARE_ENCRYPTION_KEY environment variable must be set (64 hex chars = 32 bytes)")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return fmt.Errorf("SHARE_ENCRYPTION_KEY must be valid hex: %w", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("SHARE_ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	shareEncryptionKey = key
	return nil
}

// encryptShare encrypts a share using AES-256-GCM.
// Returns base64-encoded nonce+ciphertext.
func encryptShare(plaintext string) (string, error) {
	block, err := aes.NewCipher(shareEncryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptShare decrypts a base64-encoded AES-256-GCM ciphertext.
func decryptShare(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(shareEncryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt share: %w", err)
	}

	return string(plaintext), nil
}
