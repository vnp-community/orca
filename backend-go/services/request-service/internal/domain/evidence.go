package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

func NewEvidence(id, tenantID, requestID string, seq int, sourceID, sourceRef, title, excerpt, digest string, retrievedAt time.Time) Evidence {
	if len(excerpt) > 4096 {
		excerpt = excerpt[:4096]
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
