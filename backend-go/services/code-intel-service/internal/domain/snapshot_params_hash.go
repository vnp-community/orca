package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CanonicalParamsHash generates a stable SHA-256 hash for query parameters.
func CanonicalParamsHash(params any) (string, error) {
	// json.Marshal provides a consistent output for structs.
	// For a more robust canonicalization, it could sort map keys (which json.Marshal does)
	// and handle omit-empty for defaults.
	b, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	
	// Prefix with GraphModelVersion to ensure hash changes if data shape changes
	prefix := []byte(GraphModelVersion + ":")
	hash := sha256.Sum256(append(prefix, b...))
	
	return hex.EncodeToString(hash[:]), nil
}
