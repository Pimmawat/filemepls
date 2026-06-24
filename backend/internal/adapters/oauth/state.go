package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// NewState generates a random, URL-safe CSRF state value to embed in an
// OAuth authorize request and verify on callback.
func NewState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
