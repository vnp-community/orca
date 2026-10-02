package domain

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// ErrInvalidAuditPageToken is returned for a malformed newest-first page token.
var ErrInvalidAuditPageToken = errors.New("domain: invalid audit page token")

// EncodeAuditKeyset makes the "<rfc3339nano utc>|<id>" token that resumes a
// newest-first listing after e. Timestamps are taken from the stored row so
// the comparison in SQL is exact.
func EncodeAuditKeyset(e AuditEntry) string {
	return e.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + e.ID
}

// DecodeAuditKeyset parses a token produced by EncodeAuditKeyset.
func DecodeAuditKeyset(token string) (time.Time, string, error) {
	i := strings.LastIndexByte(token, '|')
	if i <= 0 || i == len(token)-1 {
		return time.Time{}, "", ErrInvalidAuditPageToken
	}
	at, err := time.Parse(time.RFC3339Nano, token[:i])
	if err != nil {
		return time.Time{}, "", ErrInvalidAuditPageToken
	}
	return at, token[i+1:], nil
}

// SortedKeys returns the map's keys in a stable order (deterministic SQL).
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// AuditOrderBy is the ORDER BY clause for the two supported orders.
func AuditOrderBy(newestFirst bool) string {
	if newestFirst {
		return "occurred_at DESC, id DESC"
	}
	return "id"
}
