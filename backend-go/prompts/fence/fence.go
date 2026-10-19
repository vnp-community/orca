// Package fence keeps text written by an outside party from closing the block it is wrapped in or from growing without bound.
package fence

import (
	"strings"
	"unicode/utf8"
)

// blockTags are the blocks whose closing tag untrusted text must not be able to write.
var blockTags = []string{"request", "prior_artifacts", "clarifications", "feedback", "project"}

// Untrusted neutralises the closing tag of every block that wraps user-written text, so the text cannot end the block early.
func Untrusted(s string) string {
	for _, tag := range blockTags {
		s = Closing(s, tag)
	}
	return s
}

// Closing neutralises the closing tag of one block. A block with its own tag (approved_decision) uses it directly.
func Closing(s, tag string) string {
	return strings.ReplaceAll(s, "</"+tag, "<\\/"+tag)
}

// Truncate keeps the first n runes. It counts runes, never bytes, so a multi-byte character is not cut in half.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
