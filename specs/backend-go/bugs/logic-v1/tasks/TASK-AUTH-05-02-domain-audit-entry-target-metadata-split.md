# TASK-AUTH-05-02: `domain.AuditEntry`/`NewAuditEntry` split `Target` into `TargetType`/`TargetID`/`Metadata`/`IPAddress`

**From Solution:** SOL-AUTH-05
**Priority:** P0
**Service:** `auth-service` (domain)
**File:** `backend-go/services/auth-service/internal/domain/audit.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`AuditEntry.Target` is a single string today; `auth-service.md` §4 already specifies the split (`resourceType`/`resourceID`/structured `payload`). This is a breaking signature change to `NewAuditEntry` across every existing call site (handled in TASK-AUTH-05-05) — unavoidable given the fields being added are exactly what every call site under-specifies today.

## Changes to make

In `backend-go/services/auth-service/internal/domain/audit.go`, change the `AuditEntry` struct and `NewAuditEntry`:

```go
package domain

import (
	"errors"
	"time"
)

var (
	ErrEmptyAction    = errors.New("domain: action is required")
	ErrZeroOccurredAt = errors.New("domain: occurred_at is required")
)

// AuditEntry is one row of auth-service's append-only, system-wide
// security-audit record. ActorID may be empty for a system-initiated event.
// TargetType/TargetID may also both be empty for a system-initiated event
// with no single resource target (e.g. the session reaper's batch purge).
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
```

Do not update any usecase call sites in this task — that is TASK-AUTH-05-05. This task will leave the package non-building until that task lands; that is expected.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/internal/domain/...
go test ./services/auth-service/internal/domain/... -run TestAuditEntry -v
```

Expected: `domain` package itself builds and its own tests pass (`go build ./services/auth-service/...` as a whole will fail until TASK-AUTH-05-05 updates call sites — that's expected at this step). Test cases: `NewAuditEntry` with `nil` metadata → normalizes to `{}`, not `nil`; empty `targetType`/`targetID` still constructs successfully; existing invariant tests (`ErrEmptyID`, `ErrEmptyTenant`, `ErrEmptyAction`, `ErrZeroOccurredAt`) still pass against the new signature.
