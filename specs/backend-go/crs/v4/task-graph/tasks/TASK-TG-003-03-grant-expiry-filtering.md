# TASK-TG-003-03: Grant expiry (`Grant.ExpiresAt`, filtered at read time)

**From Solution:** BE-SOL-003
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/grant.go`, `backend-go/services/task-service/internal/usecase/resolve_permission.go`, `backend-go/services/task-service/migrations/0004_task_fields_and_comments.up/down.sql` (append `grants.expires_at`, shared file — see Context)
**Depends on:** TASK-TG-001-01 (shares the same migration file — see Context)
**Status:** `[x]` DONE

---

## Context

Verified directly (`internal/domain/grant.go:1-96`, read in full): `Grant`
today is `{TaskID, SubjectID, Level, ApplyTree}` (lines 54-59) — no
`ExpiresAt` field. `ResolvePermission.Execute`
(`internal/usecase/resolve_permission.go:51-91`, read in full) builds
`grantsByTask` at line 66 (`uc.grants.ListGrantsForAncestors(...)`) and
passes it straight to `domain.ResolveGrant` at line 77 with no filtering
step in between today.

**Migration file coordination**: this task's `ALTER TABLE task.grants ADD
COLUMN expires_at TIMESTAMPTZ` must land in the SAME
`0004_task_fields_and_comments.up.sql` file TASK-TG-001-01 creates — per
BE-SOL-003's own affected-files note and TASK-TG-001-01's Context section.
If TASK-TG-001-01 has already landed by the time this task starts, append a
new statement to the existing file rather than creating `0005` (a migration
file, once applied to any environment, should not be edited after the
fact — if `0004` is already applied anywhere, this task must instead create
a genuinely new next-free-numbered migration; check
`ls backend-go/services/task-service/migrations/` at implementation time to
decide which applies). The real table name is `task.task_grants` (confirmed,
`migrations/0001_init.up.sql:67-76`), not bare `grants` as BE-SOL-003's
prose shorthand says.

## Changes to make

**1. Migration** — append to `0004_task_fields_and_comments.up.sql` (or a
new file if `0004` is already applied — see Context):

```sql
ALTER TABLE task.task_grants ADD COLUMN expires_at TIMESTAMPTZ;
```

down:

```sql
ALTER TABLE task.task_grants DROP COLUMN IF EXISTS expires_at;
```

**2. `internal/domain/grant.go`** — add the field (additive, no enum
change, per BE-SOL-003's own correction that `GrantLevel` stays untouched):

```go
type Grant struct {
	TaskID    string
	SubjectID string
	Level     GrantLevel
	ApplyTree bool
	ExpiresAt *time.Time // NEW, nullable
}
```

**3. `internal/usecase/resolve_permission.go`** — filter expired grants
where `grantsByTask` is assembled, **before** calling `domain.ResolveGrant`
(keeping `ResolveGrant`'s own signature and existing unit tests untouched,
per BE-SOL-003's explicit design choice not to add a `now time.Time`
parameter to that pure function):

```go
grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, chain)
if err != nil {
	return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindInternal, "TASK_GRANT_LIST_FAILED", "failed to list grants for ancestor chain", err)
}
now := time.Now()
for taskID, grants := range grantsByTask {
	grantsByTask[taskID] = filterExpired(grants, now)
}
```

```go
// filterExpired drops any grant whose ExpiresAt has passed — a grant with a
// nil ExpiresAt never expires.
func filterExpired(grants []domain.Grant, now time.Time) []domain.Grant {
	out := grants[:0]
	for _, g := range grants {
		if g.ExpiresAt == nil || g.ExpiresAt.After(now) {
			out = append(out, g)
		}
	}
	return out
}
```

(Insert this right after the `ListGrantsForAncestors` call at line 66-69,
before the `TeamScopeResolver.ResolveTeams` call at line 71 — order doesn't
matter functionally, but keeping it adjacent to the grants-fetch call keeps
the filtering step visually scoped to what it operates on.)

**4. `internal/adapter/postgres/grants.go`** (75 lines total, confirmed by
direct read) — `Grant` (lines 26-39) and `ListGrantsForAncestors` (lines
41-74) both need the new column:

```go
// Grant — widen the INSERT (line 31-34) and its param list:
_, err := r.db.Exec(ctx, `
	INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level, apply_tree, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6)
`, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree, grant.ExpiresAt)
```

```go
// ListGrantsForAncestors — widen the SELECT (line 52) and Scan (line 64):
SELECT task_id, subject_id, level, apply_tree, expires_at FROM task.task_grants WHERE tenant_id = $1 AND task_id = ANY($2)
// ...
rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt)
```

**5. `usecase.Grant`/`GrantInput`** (`internal/usecase/grant.go:11-16`) and
the `Grant` RPC's wire message (`task.proto`'s `GrantRequest`, currently 4
fields — `task_id=1, subject_id=2, level=3, apply_tree=4`, confirmed by
direct read of `task.proto:104-109`) gain an optional `expires_at` field
too, so a grant can actually be created with an expiry — BE-SOL-003's design
section only covers the read-time filter, but a field with no write path is
dead; add `ExpiresAt *time.Time` to `GrantInput` and
`google.protobuf.Timestamp expires_at = 5;` to `GrantRequest` in the same
task, since leaving it write-only-via-direct-SQL would make this feature
untestable through the RPC surface.

## Test plan

- Expired grant (`ExpiresAt` in the past) excluded from `grantsByTask`
  before `ResolveGrant` runs — a caller with ONLY an expired grant resolves
  `Unspecified`/deny.
- A grant with `ExpiresAt` in the future, or nil, still resolves normally
  (regression test that this change doesn't affect non-expiring grants).
- `Grant` RPC with `expires_at` set persists it; `ListGrants`
  (TASK-TG-003-04) returns it back unchanged.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestResolvePermission|TestGrant" -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; expired-grant test asserts `PermissionDenied`/
`TASK_NO_GRANT`, matching the identical error `ResolvePermission` already
returns for "no grant at all" (per that usecase's own doc comment on not
leaking which case applies).

## Execution notes (2026-09-09)

Migration already landed as part of TASK-TG-001-01's combined
`0004_task_fields_and_comments.up/down.sql` (per that task's own
coordination note — `task.task_grants.expires_at TIMESTAMPTZ`, nullable, no
new migration file created here). Implemented every other piece exactly per
the task's own code samples: `domain.Grant.ExpiresAt *time.Time`;
`resolve_permission.go`'s `filterExpired` helper inserted right after the
`ListGrantsForAncestors` call, `domain.ResolveGrant`'s signature left
untouched; `grants.go`'s `Grant`/`ListGrantsForAncestors` widened for the
new column; `usecase.GrantInput.ExpiresAt` + `GrantRequest.expires_at = 5`
(a `google.protobuf.Timestamp`, regenerated via `buf generate`) so the
field actually has a write path through the RPC, not just direct-SQL, per
this task's own "a field with no write path is dead" note; `server.go`'s
`Grant` handler maps the wrapper's `AsTime()` when set.

Added exactly the 3 cases the task's own Test plan names:
`TestResolvePermission_ExpiredGrant_ExcludedFromResolution` (expired-only
grant → `PermissionDenied`, OPA never even called);
`TestResolvePermission_NonExpiringAndFutureExpiry_StillResolve` (table test,
nil and future `ExpiresAt` both resolve normally — the regression guard
that non-expiring grants are unaffected); `TestGrant_PersistsExpiresAt` (the
RPC write-path test — a grant created with `ExpiresAt` set persists it
unchanged through the fake `GrantRepository`, and by extension through
`grants.go`'s widened INSERT).

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go
test .../usecase/... -run "TestResolvePermission|TestGrant"` — all 12 cases
pass (8 pre-existing + 4 new); `go test -tags=integration
.../postgres/... -run TestRepository_Grant_And_ListGrantsForAncestors` —
hit the same pre-existing testcontainers flake on the first run (documented
in TASK-TG-001-02's execution notes), passed cleanly (9.21s) on immediate
re-run, confirming the widened `Grant`/`ListGrantsForAncestors` SQL against
a real `expires_at` column; full `go test ./services/task-service/...`
passes with no regressions.
