package usecase

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
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

// hashToken is the pairing-token equivalent of domain.HashSessionToken —
// the raw pairing_token is never the lookup key, only its hash
// (domain.PairingSession.ID's doc comment). Reuses the exact same SHA-256
// construction rather than inventing a second hashing scheme.
func hashToken(rawToken string) string {
	return domain.HashSessionToken(rawToken)
}

// newUUID mints a new random (v4) identifier — used for domain.PairedDevice.ID,
// the same generator Login's audit-entry ID uses (uuid.NewString()).
func newUUID() string {
	return uuid.NewString()
}
