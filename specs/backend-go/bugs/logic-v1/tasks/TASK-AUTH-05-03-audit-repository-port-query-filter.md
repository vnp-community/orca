# TASK-AUTH-05-03: `AuditRepository.Query` gains a filter struct (`action`/`actor_id`/`to`)

**From Solution:** SOL-AUTH-05
**Priority:** P0
**Service:** `auth-service` (usecase port)
**File:** `backend-go/services/auth-service/internal/usecase/ports.go`
**Depends on:** TASK-AUTH-05-02
**Status:** `[x]` DONE — `AuditRepository.Query` widened to `AuditQueryFilter`; confirmed the expected build breakage points only at call-site/implementation mismatches (postgres adapter, `query_audit_log.go`), no struct-field typo.

---

## Context

`AuditRepository.Query` currently takes fixed positional params (`tenantID, since, pageToken, pageSize`) with no way to filter by action, actor, or an upper time bound. The admin console's audit-log view needs `action`/`userId`/`to` query params (BUG-AUTH-05's spec). This widens the port to a filter struct so the number of optional predicates can grow without another signature break.

## Changes to make

In `backend-go/services/auth-service/internal/usecase/ports.go`, change the `AuditRepository` interface:

```go
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
	Query(ctx context.Context, filter AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error)
}

// AuditQueryFilter narrows a Query call. Zero-value fields (empty string,
// zero time.Time) mean "no filter on this dimension" — only TenantID and
// Since are ever required by a real caller today.
type AuditQueryFilter struct {
	TenantID string
	Since    time.Time
	To       time.Time // zero value = no upper bound
	Action   string    // "" = no filter
	ActorID  string    // "" = no filter
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/internal/usecase/... 2>&1 | head -50
```

Expected: this will fail to build until the postgres adapter (TASK-AUTH-05-04) and `query_audit_log.go` (TASK-AUTH-05-06) are updated to the new signature — that's expected at this step; confirm the compiler errors point only at call-site/implementation mismatches, not a struct-field typo.
