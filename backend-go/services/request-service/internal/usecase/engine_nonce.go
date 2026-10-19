package usecase

import (
	"crypto/rand"
	"encoding/hex"
)

// randomNonce is 32 hex characters: inside the agent's 16..64 alphanumeric rule, and unguessable by text the run reads.
func randomNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("usecase: no randomness for a result nonce: " + err.Error())
	}
	return hex.EncodeToString(b)
}
