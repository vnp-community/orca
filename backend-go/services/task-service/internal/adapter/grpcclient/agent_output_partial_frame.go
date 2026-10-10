package grpcclient

import (
	"strings"
	"unicode/utf8"
)

// maxPartialStreamBytes bounds one stream of an agent_output_partial frame. Every frame repeats the whole run
// so far and travels through one NATS message (1 MiB default limit): an oversized frame would block the outbox.
const maxPartialStreamBytes = 128 << 10

// boundPartialStream keeps the tail of s (the newest output) within maxPartialStreamBytes and removes what the
// jsonb outbox column rejects: NUL bytes and invalid UTF-8, including a cut through a multi-byte character.
func boundPartialStream(s string) string {
	s = strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
	if len(s) <= maxPartialStreamBytes {
		return s
	}
	cut := len(s) - maxPartialStreamBytes
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	return s[cut:]
}
