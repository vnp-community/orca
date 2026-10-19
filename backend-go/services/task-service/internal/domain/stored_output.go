package domain

import (
	"strings"
	"unicode/utf8"
)

// BoundStoredOutput prepares agent stdout for a TEXT column: Postgres rejects invalid UTF-8 and NUL bytes, and a
// cut in the middle of a character is invalid UTF-8, so a long non-ASCII output would otherwise be lost whole.
func BoundStoredOutput(s string, maxBytes int) string {
	s = strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
