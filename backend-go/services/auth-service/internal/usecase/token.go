package usecase

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// generateRandomToken returns a high-entropy, URL-safe random string —
// used both for session tokens (auth-service.md §4: "not a JWT") and for
// the placeholder initial password CreateUser generates (see that
// usecase's doc comment). n is the number of random bytes read.
func generateRandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("usecase: generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
