package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Evidence struct {
	ID          string
	TenantID    string
	RequestID   string
	Seq         int
	SourceID    string
	SourceRef   string
	Title       string
	Excerpt     string
	Digest      string
	RetrievedAt time.Time
	UsedBy      []EvidenceUse
}

type EvidenceUse struct {
	Kind string // solution, plan, task, assessment
	ID   string
}

func EvidenceRef(seq int) string {
	return fmt.Sprintf("EVD-%d", seq)
}

func ParseEvidenceRef(s string) (int, bool) {
	if !strings.HasPrefix(s, "EVD-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "EVD-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// evidenceExcerptMaxBytes matches CHECK (octet_length(excerpt) <= 4096) in migration 0005.
const evidenceExcerptMaxBytes = 4096

func NewEvidence(id, tenantID, requestID string, seq int, sourceID, sourceRef, title, excerpt, digest string, retrievedAt time.Time) Evidence {
	// The column limit is in bytes (octet_length <= 4096), so cut by bytes but back up to a rune boundary to keep the text valid UTF-8.
	if len(excerpt) > evidenceExcerptMaxBytes {
		cut := evidenceExcerptMaxBytes
		for cut > 0 && !utf8.RuneStart(excerpt[cut]) {
			cut--
		}
		excerpt = excerpt[:cut]
	}
	return Evidence{
		ID:          id,
		TenantID:    tenantID,
		RequestID:   requestID,
		Seq:         seq,
		SourceID:    sourceID,
		SourceRef:   sourceRef,
		Title:       title,
		Excerpt:     excerpt,
		Digest:      digest,
		RetrievedAt: retrievedAt,
	}
}
