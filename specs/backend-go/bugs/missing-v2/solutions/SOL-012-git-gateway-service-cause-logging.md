# SOL-012: Wire `apperrors.SetLogger` into `git-gateway-service` (extends SOL-009 to a third service)

> **✅ IMPLEMENTED & DEPLOYED (2026-09-15)** — `sync-to-server.sh 2026.09.15-batch2-fix`. Live-confirmed: `docker logs orca-go-git-gateway` already shows a real `"apperrors: internal cause"` line for an unrelated error (`WORKTREE_REPO_NOT_FOUND`/`PROJECT_REPO_FETCH_FAILED`) minutes after deploy — the logging wiring works. Still waiting on a live `GITGATEWAY_STATUS_FAILED` recurrence specifically to close BUG-012 with its real cause.

## Bug Reference
- **Bug:** BUG-012
- **Severity:** Medium
- **Pattern:** identical to [SOL-009](./SOL-009-apperrors-optional-cause-logging.md) (`infra-fleet-service`) and the wiring half of [SOL-008](./SOL-008-profile-resolver-forward-tenant-metadata.md)'s follow-on family — this is pure wiring, `common/apperrors`'s `SetLogger`/`ToGRPCStatus` already support this with zero further code changes.

---

## Root Cause

See [BUG-012](../BUG-012-gitgateway-status-failed-opaque-relay-error.md). `apperrors.SetLogger` is only called in `infra-fleet-service/cmd/server/main.go` — every other service (including `git-gateway-service`) never wires it, so any `AppError`'s wrapped cause raised in those services is discarded before `ToGRPCStatus` converts it to a client-safe status, with no server-side trace of what the real cause was.

## Fix

**File:** `backend-go/services/git-gateway-service/cmd/server/main.go`

Add, immediately after `logger := logging.New(...)` (mirroring `infra-fleet-service/cmd/server/main.go`'s exact placement — see SOL-009/TASK-018):

```go
import "github.com/stablyai/orca-go/common/apperrors"
...
apperrors.SetLogger(logger)
```

No other change. `apperrors.ToGRPCStatus` (already deployed, SOL-009) is nil-safe when no logger is set — this call only starts populating `"apperrors: internal cause (not sent to client)"` log lines for `git-gateway-service`'s own `AppError`s going forward; the client-visible gRPC status text is byte-for-byte unchanged (verified by SOL-009's own tests, which this change doesn't need to duplicate — same function, same contract, only a different service now calling `SetLogger`).

## Impact Analysis

Not yet run (`gitnexus impact({target: "main", ...})` on `git-gateway-service`'s `main.go` isn't meaningful — this is a one-line addition to a `func main()`/`func run()` already calling several other package-level setup functions of this exact shape). SOL-009's own impact analysis on `ToGRPCStatus` (`CRITICAL`, 463 callers) already covered the shared function this reuses; this SOL adds a caller, not a new code path.

## Why extend service-by-service instead of wiring it globally once

Same reasoning as SOL-009's original design and BUG-010's fix: `apperrors.SetLogger` is a **package-level, process-wide** setter — there's no way to wire it "once for everyone" across separate service binaries without a shared bootstrap helper, which doesn't currently exist in this codebase's `cmd/server/main.go` pattern (each service's `main.go` independently constructs its own logger, config, and dependencies). Introducing such a shared helper is a reasonable follow-up (see "Worth considering" below) but out of scope for this minimal, low-risk fix.

## Worth considering (not part of this SOL)

This is now the **third** service (`infra-fleet-service`, `project-service` implicitly via BUG-010 investigation, now `git-gateway-service`) where the exact same "is the observability logger wired here?" question has come up. Per this bug family's established pattern of naming a structural fix once the same gap repeats (see `missing-v2/solutions/README.md`'s "Cross-cutting design theme" for BUG-001/005/006), this is worth flagging as a candidate for a shared `common/serverboot` (or similar) helper that every `cmd/server/main.go` calls, wiring `apperrors.SetLogger` (and any other process-wide, additive-only setup) automatically — so the next new service doesn't silently start this same gap over again. Not designed here; noted for whoever picks this up.

## Testing (planned, not yet written)

Same shape as `apperrors_test.go` (SOL-009) — no new tests needed for `apperrors` itself (unchanged); `git-gateway-service`'s own test suite doesn't need a new test for this one-line wiring (nothing to unit-test beyond "was `SetLogger` called," which isn't a meaningful assertion — same reasoning SOL-009/TASK-018 applied when wiring `infra-fleet-service`).

## Deploy status

Not deployed — code not yet written, per this session's explicit "log the bug first" instruction. Once implemented: standard `sync-to-server.sh` run (rebuilds all services; only `git-gateway-service`'s image changes). After deploy, re-trigger the `GITGATEWAY_STATUS_FAILED` failure and read `docker logs orca-go-git-gateway` for the new `"apperrors: internal cause"` line — update BUG-012.md with the real cause once seen, exactly as BUG-009 was updated after SOL-009's deploy exposed its real cause.
