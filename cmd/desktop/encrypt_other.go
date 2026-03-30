//go:build !windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// getOrCreateEncryptionKey returns a 32-byte AES key, generating one on first use.
// The key is stored in a separate file with 0400 permissions.
func getOrCreateEncryptionKey() ([]byte, error) {
	keyPath := filepath.Join(getConfigDir(), ".keyfile")

	key, err := os.ReadFile(keyPath)
	if err == nil && len(key) == 32 {
		return key, nil
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}

	if err := os.WriteFile(keyPath, key, 0400); err != nil {
		return nil, fmt.Errorf("write encryption key: %w", err)
	}

	return key, nil
}

func encryptCredential(plaintext []byte) ([]byte, error) {
	key, err := getOrCreateEncryptionKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptCredential(ciphertext []byte) ([]byte, error) {
	key, err := getOrCreateEncryptionKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
