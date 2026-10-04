# SOL-009: `common/apperrors.ToGRPCStatus` logs an `AppError`'s wrapped cause to an optional, nil-by-default logger before discarding it

**Resolves:** BUG-009 (the "Observability gap" section — not the relay-failure itself, which is not being fixed, see BUG-009's "Resolution")
**Service:** `common/apperrors` (shared by all 16 gRPC-serving services), wired in `infra-fleet-service`
**Affected files:** `common/apperrors/apperrors.go`, `services/infra-fleet-service/cmd/server/main.go`
**Priority:** Low (pure observability improvement, not a correctness fix)
**Status:** ✅ IMPLEMENTED (2026-09-14) — TASK-018 + TASK-019

---

## Why this exists

Investigating BUG-009 live against `b15.openledger.vn` found that `ToGRPCStatus` (`apperrors.go:70-98` as it existed before this fix) builds the client-facing status as `ae.Code+": "+ae.Message` only — `ae.Err` (the real underlying cause, e.g. a `devserveragent` transport failure) is silently discarded. This is correct **for the client** (per this function's own doc comment — clients must key off the stable `Code`, never off internal detail), but it also meant the cause never reached even the service's **own** structured logs: `common/grpcmw.LoggingInterceptor` logs `err.Error()` on the value `ToGRPCStatus` *returns* — by which point the cause is already gone. Confirmed directly: reading `orca-go-infra-fleet`'s raw container logs at the exact failure timestamp showed no line anywhere carrying more detail than the client already saw. **This class of error was undiagnosable from logs alone, even with full SSH+log access to the live deployment.**

## Design

Package-level, nil-by-default logger — same wire-once-from-main convention already established in this codebase for `project-service/internal/usecase/authorization.go`'s `auditClient` (a deliberate precedent, not a new pattern):

```go
// common/apperrors/apperrors.go
var logger *slog.Logger

func SetLogger(l *slog.Logger) { logger = l }

func ToGRPCStatus(err error) error {
	...
	if ae.Err != nil && logger != nil {
		logger.Error("apperrors: internal cause (not sent to client)",
			"code", ae.Code,
			"cause", ae.Err.Error(),
		)
	}
	...
	return status.Error(code, ae.Code+": "+ae.Message)  // unchanged
}
```

### Why this shape, given `ToGRPCStatus` has 463 direct callers (CRITICAL impact)

- **No signature change** — every one of the 463 existing `apperrors.ToGRPCStatus(err)` call sites across 16 services needed zero edits.
- **`logger == nil` by default** — a service that never calls `SetLogger` (i.e. every service except `infra-fleet-service`, for now) gets byte-for-byte the same return value as before this change existed. Verified: `go build`/`go vet`/`go test` clean across all 20 `go.work` modules.
- **Client-visible behavior is unchanged either way** — the returned `status.Error(code, ae.Code+": "+ae.Message)` line is untouched; only a new server-side-only log line is added when a logger is wired AND the `AppError` actually carries a wrapped cause.
- **Scoped to `infra-fleet-service` only, for now** — matches BUG-009's actual scope. Wiring the other 15 services is optional, straightforward follow-up (one `apperrors.SetLogger(logger)` line each, same as this one) — not required to close BUG-009, since the specific error that prompted this investigation is `infra-fleet-service`'s.

## Testing

`common/apperrors/apperrors_test.go` (new — no test file existed for this package before this task):
- `TestToGRPCStatus_NoLoggerWired_Unaffected` — asserts the returned gRPC status is byte-for-byte the same as before (code + client-safe message, cause never appears) when no logger is wired — the regression guard for "463 existing callers must be unaffected."
- `TestToGRPCStatus_LoggerWired_LogsCauseServerSideOnly` — asserts the wrapped cause DOES appear in the server-side log once `SetLogger` is called, while the client-visible message still excludes it.
- `TestToGRPCStatus_NoWrappedCause_NeverLogs` — guards against a spurious log line for the common case of an `AppError` with no wrapped `Err` at all.

## Verify

```bash
cd backend-go
go build ./common/... ./services/infra-fleet-service/...
go vet ./common/... ./services/infra-fleet-service/...
go test ./common/apperrors/... ./services/infra-fleet-service/... -count=1
```

Also verified: `go build`/`go vet` clean for **all 20** `go.work` modules (not just the two touched), confirming the shared-package change doesn't break any of the other 14 services that call `ToGRPCStatus` without wiring a logger.
