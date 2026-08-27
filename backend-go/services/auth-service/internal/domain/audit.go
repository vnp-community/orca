package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyAction is returned when an AuditEntry is constructed with no
	// action — an audit entry that doesn't say what happened is useless.
	ErrEmptyAction = errors.New("domain: action is required")
	// ErrZeroOccurredAt is returned when OccurredAt is unset.
	ErrZeroOccurredAt = errors.New("domain: occurred_at is required")
)

// AuditEntry is one row of auth-service's append-only, system-wide
// security-audit record (auth-service.md §4). ActorID may be empty for a
// system-initiated event (e.g. the session reaper revoking an expired
// session) — that is intentionally not an invariant violation.
//
// Immutable once written: there is deliberately no usecase method that
// updates or deletes an AuditEntry, only Append and Query.
type AuditEntry struct {
	ID         string
	TenantID   string
	ActorID    string
	Action     string
	Target     string
	OccurredAt time.Time
}

// NewAuditEntry constructs an AuditEntry, enforcing that every entry has an
// action and a timestamp — the two fields an audit record is meaningless
// without.
func NewAuditEntry(id, tenantID, actorID, action, target string, occurredAt time.Time) (AuditEntry, error) {
	if id == "" {
		return AuditEntry{}, ErrEmptyID
	}
	if tenantID == "" {
		return AuditEntry{}, ErrEmptyTenant
	}
	if action == "" {
		return AuditEntry{}, ErrEmptyAction
	}
	if occurredAt.IsZero() {
		return AuditEntry{}, ErrZeroOccurredAt
	}
	return AuditEntry{
		ID:         id,
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     action,
		Target:     target,
		OccurredAt: occurredAt,
	}, nil
}
