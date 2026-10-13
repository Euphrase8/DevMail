// Package secretbox encrypts sensitive values (provider credentials, DKIM
// private keys) at rest using AES-256-GCM. The key is derived from the
// configured passphrase with SHA-256, so any passphrase length is accepted.
// Sealed values are base64(nonce || ciphertext) and never logged.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// deriveKey returns the 32-byte AES-256 key for a passphrase.
func deriveKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}

// Seal encrypts plaintext with the passphrase-derived key.
func Seal(passphrase, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := deriveKey(passphrase)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open decrypts a value produced by Seal. ErrInvalid indicates tampering or a
// wrong key; the message never leaks the underlying value.
var ErrInvalid = errors.New("invalid sealed value")

// Open decrypts a base64 value produced by Seal.
func Open(passphrase, encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalid
	}
	key := deriveKey(passphrase)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", ErrInvalid
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrInvalid
	}
	if len(raw) < gcm.NonceSize() {
		return "", ErrInvalid
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalid
	}
	return string(plain), nil
}

// Masked reports a redacted hint for a secret without revealing it.
func Masked(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}
