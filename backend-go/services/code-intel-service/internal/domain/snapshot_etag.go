package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ContentETag returns the ETag based on the content hash.
func ContentETag(contentHash string) string {
	return fmt.Sprintf("\"%s\"", contentHash)
}

// ClientETag combines the content ETag, head commit, and staleness into a single ETag.
func ClientETag(contentETag string, head string, stale bool) string {
	tag := strings.Trim(contentETag, "\"")
	staleStr := "0"
	if stale {
		staleStr = "1"
	}
	
	raw := fmt.Sprintf("%s-%s-%s", tag, head, staleStr)
	hash := sha256.Sum256([]byte(raw))
	
	// ETag is max 32 characters, so we take the first 16 bytes of the hash (32 hex chars)
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:16]))
}

// ValidateIfNoneMatch checks if the If-None-Match header is within the allowed length limit.
func ValidateIfNoneMatch(ifNoneMatch string) error {
	if len(ifNoneMatch) > 80 {
		return errors.New("CODEINTEL_INVALID_PARAMS: if_none_match exceeds 80 characters")
	}
	return nil
}
