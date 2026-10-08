package domain

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// ParseStatus says whether the agent's result block was found and well formed.
type ParseStatus string

const (
	ParseStatusOK      ParseStatus = "ok"
	ParseStatusMissing ParseStatus = "missing"
	ParseStatusInvalid ParseStatus = "invalid"
)

func (p ParseStatus) Valid() bool {
	return p == ParseStatusOK || p == ParseStatusMissing || p == ParseStatusInvalid
}

// MaxStdoutTailBytes keeps the stored tail small; the full stdout is never persisted here.
const MaxStdoutTailBytes = 16 * 1024

// ExecutionRecord is the structured outcome of one contract run of a task (CR-REQ-029).
// The digests are filled by request-service callers when known; this service leaves them empty.
type ExecutionRecord struct {
	ID              string
	TenantID        string
	TaskID          string
	ExecutionLinkID string
	Attempt         int
	SpecDigest      string
	PacketDigest    string
	TemplateVersion string
	ParseStatus     ParseStatus
	FailureClass    FailureClass
	Result          []byte
	Changes         []byte
	StdoutTail      string
	CreatedAt       time.Time
}

// TailUTF8 keeps the last max bytes of s, cut on a rune boundary so the column never holds a
// broken character. NUL bytes are dropped because Postgres text rejects them.
func TailUTF8(s string, max int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if max <= 0 {
		return ""
	}
	if len(s) > max {
		start := len(s) - max
		for start < len(s) && !utf8.RuneStart(s[start]) {
			start++
		}
		s = s[start:]
	}
	return strings.ToValidUTF8(s, "")
}

// StorableJSON returns raw when it can live in a JSON column and nil otherwise: JSONB refuses
// the \u0000 escape and any invalid document, and one bad agent byte must not lose the record.
func StorableJSON(raw []byte) []byte {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	if bytes.Contains(bytes.ToLower(raw), []byte(`\u0000`)) {
		return nil
	}
	return raw
}
