package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 4
	argonSaltLen     = 16
	argonKeyLen      = 32
)

// HashPassword derives an Argon2id hash and encodes it in a self-describing
// format: argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>.
func HashPassword(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// ParseArgonHash splits an encoded Argon2id hash into its parameters.
// Format: argon2id$v=19$m=65536,t=3,p=4$<saltB64>$<keyB64>
func ParseArgonHash(hash string) (memory, iterations, parallelism uint32, salt, key []byte, ok bool) {
	parts := strings.Split(hash, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || !strings.HasPrefix(parts[1], "v=") {
		return 0, 0, 0, nil, nil, false
	}
	version := 0
	if _, err := fmt.Sscanf(parts[1], "v=%d", &version); err != nil || version != 19 {
		return 0, 0, 0, nil, nil, false
	}
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return 0, 0, 0, nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return 0, 0, 0, nil, nil, false
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(key) != argonKeyLen {
		return 0, 0, 0, nil, nil, false
	}
	return memory, iterations, parallelism, salt, key, true
}

// VerifyPassword checks a password against an encoded Argon2id hash.
func VerifyPassword(hash, password string) bool {
	memory, iterations, parallelism, salt, want, ok := ParseArgonHash(hash)
	if !ok {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), argonKeyLen)
	return sha256.Sum256(got) == sha256.Sum256(want)
}

// NewSessionToken returns a URL-safe random session token.
func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashSessionToken hashes a session token for storage so that a database
// leak does not expose usable session tokens.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}