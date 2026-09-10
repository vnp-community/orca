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
	// ErrInvalidOutcome is returned when Outcome is set to a value outside
	// the closed enum (OutcomeAllowed/OutcomeDenied).
	ErrInvalidOutcome = errors.New("domain: invalid outcome")
)

// Outcome records whether the audited action was allowed or denied —
// CR-RBAC-005/TASK-BE-014, so an audit entry can distinguish "denied" from
// "allowed" instead of only ever recording successes.
type Outcome string

const (
	OutcomeAllowed Outcome = "allowed"
	OutcomeDenied  Outcome = "denied"
)

func (o Outcome) Valid() bool {
	switch o {
	case OutcomeAllowed, OutcomeDenied:
		return true
	default:
		return false
	}
}

// AuditEntry is one row of auth-service's append-only, system-wide
// security-audit record (auth-service.md §4). ActorID may be empty for a
// system-initiated event. TargetType/TargetID may also both be empty for a
// system-initiated event with no single resource target (e.g. the session
// reaper's batch purge).
//
// Immutable once written: there is deliberately no usecase method that
// updates or deletes an AuditEntry, only Append and Query.
type AuditEntry struct {
	ID       string
	TenantID string
	ActorID  string
	Action   string
	// Target is the legacy single-string target identifier — superseded by
	// TargetType/TargetID below (SOL-AUTH-05), kept during the transition
	// window for old rows and for AppendAuditEntry's cross-service callers
	// (which only carry a flat target string, no type/id split). Never
	// populated by auth-service's own usecases going forward.
	Target     string
	TargetType string // "user" | "session" | "ssh_host" | ...
	TargetID   string
	Metadata   map[string]any // JSON-serializable; redacted of secret material
	Outcome    Outcome        // defaults to OutcomeAllowed for pre-existing call sites, see NewAuditEntry
	// IPAddress is empty for an entry appended by a service with no
	// HTTP-request context (e.g. a background job) — see
	// common/tenant.ClientIP's doc comment (TASK-BE-023).
	IPAddress  string
	OccurredAt time.Time
}

// NewAuditEntry constructs an AuditEntry, enforcing that every entry has an
// action and a timestamp — the fields an audit record is meaningless
// without. target/targetType/targetID are NOT required — a system-initiated
// event (the reaper, bootstrap) may have none of them, matching ActorID's
// existing "may be empty" allowance; auth-service's own usecases pass
// target="" and set targetType/targetID instead (SOL-AUTH-05), while
// AppendAuditEntry's cross-service callers pass target and leave
// targetType/targetID empty. A nil metadata is normalized to an empty map so
// downstream json.Marshal never produces "null". outcome defaults to
// OutcomeAllowed when empty, so every call site that predates CR-RBAC-005 (a
// plain "" argument) keeps its prior meaning without a mechanical edit
// beyond adding the new parameters.
func NewAuditEntry(id, tenantID, actorID, action, target, targetType, targetID string, metadata map[string]any, outcome Outcome, ipAddress string, occurredAt time.Time) (AuditEntry, error) {
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
	if outcome == "" {
		outcome = OutcomeAllowed // backward-compatible default, not an error — see domain.AuditEntry's doc comment
	}
	if !outcome.Valid() {
		return AuditEntry{}, ErrInvalidOutcome
	}
	return AuditEntry{
		ID:         id,
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     action,
		Target:     target,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   metadata,
		Outcome:    outcome,
		IPAddress:  ipAddress,
		OccurredAt: occurredAt,
	}, nil
}
