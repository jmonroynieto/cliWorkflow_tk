package id

import (
	"crypto/rand"
	"fmt"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// New generates a short random workspace ID (default 4 chars, e.g. "y4f8").
func New(length int) (string, error) {
	if length <= 0 {
		length = 4
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}
