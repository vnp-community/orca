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
// system-initiated event. TargetType/TargetID may also both be empty for a
// system-initiated event with no single resource target (e.g. the session
// reaper's batch purge).
//
// Immutable once written: there is deliberately no usecase method that
// updates or deletes an AuditEntry, only Append and Query.
type AuditEntry struct {
	ID         string
	TenantID   string
	ActorID    string
	Action     string
	TargetType string         // "user" | "session" | "ssh_host" | ...
	TargetID   string
	Metadata   map[string]any // JSON-serializable; redacted of secret material
	IPAddress  string         // may be "" — not every action has a resolvable client IP
	OccurredAt time.Time
}

// NewAuditEntry constructs an AuditEntry, enforcing that every entry has an
// action and a timestamp. targetType/targetID are NOT required — a
// system-initiated event (the reaper, bootstrap) may have neither, matching
// ActorID's existing "may be empty" allowance. A nil metadata is normalized
// to an empty map so downstream json.Marshal never produces "null".
func NewAuditEntry(id, tenantID, actorID, action, targetType, targetID string, metadata map[string]any, ipAddress string, occurredAt time.Time) (AuditEntry, error) {
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
	if metadata == nil {
		metadata = map[string]any{}
	}
	return AuditEntry{
		ID: id, TenantID: tenantID, ActorID: actorID, Action: action,
		TargetType: targetType, TargetID: targetID, Metadata: metadata,
		IPAddress: ipAddress, OccurredAt: occurredAt,
	}, nil
}
