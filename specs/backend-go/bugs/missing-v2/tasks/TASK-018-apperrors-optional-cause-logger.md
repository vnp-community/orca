# TASK-018: `apperrors.ToGRPCStatus` gains an optional, nil-by-default cause logger

**From Solution:** SOL-009
**Priority:** Low
**Service:** `common/apperrors`, `infra-fleet-service`
**File:** `common/apperrors/apperrors.go`, `services/infra-fleet-service/cmd/server/main.go`
**Depends on:** none
**Status:** `[x]` DONE — `SetLogger`/`logger` added to `apperrors.go`; `ToGRPCStatus` logs `ae.Err.Error()` (with `ae.Code`) via the package logger, gated on both `ae.Err != nil` and `logger != nil`, before building the unchanged client-facing status. `infra-fleet-service/cmd/server/main.go` calls `apperrors.SetLogger(logger)` right after constructing its logger. `go build`/`go vet` clean across all 20 `go.work` modules (not just the 2 touched — verified every module builds, confirming the other 14 services' 463-minus-infra-fleet-service call sites of `ToGRPCStatus` are unaffected).

---

## Context

See BUG-009's "Observability gap" section and SOL-009's "Why this exists" — `ToGRPCStatus` discards `AppError.Err` before returning, and no log anywhere (client or server) ever saw it, making this whole class of wrapped-cause error undiagnosable from logs alone.

`impact({target: "ToGRPCStatus", direction: "upstream"})` → `risk: CRITICAL`, 478 impacted symbols / 463 direct callers — reviewed and reported to the user before editing, per this repo's mandatory impact-analysis rule. The change made is additive only (see SOL-009's "Why this shape" section for why 463 existing call sites needed zero edits).

## Changes made

`common/apperrors/apperrors.go`:
- Added `import "log/slog"`.
- Added package-level `var logger *slog.Logger` and `func SetLogger(l *slog.Logger)`.
- Inside `ToGRPCStatus`, right after the `errors.As` type-assert succeeds and before the `switch ae.Kind` block, added:
  ```go
  if ae.Err != nil && logger != nil {
      logger.Error("apperrors: internal cause (not sent to client)",
          "code", ae.Code,
          "cause", ae.Err.Error(),
      )
  }
  ```
- No other line in `ToGRPCStatus` changed — the returned `status.Error(code, ae.Code+": "+ae.Message)` is byte-for-byte identical to before.

`services/infra-fleet-service/cmd/server/main.go`:
- Added `"github.com/stablyai/orca-go/common/apperrors"` to the import block.
- Added `apperrors.SetLogger(logger)` immediately after `logger := logging.New(...)` / `slog.SetDefault(logger)`, before any gRPC traffic is served.

## Verify

```bash
cd backend-go
go build ./common/... ./services/infra-fleet-service/...
go vet ./common/... ./services/infra-fleet-service/...
go test ./common/apperrors/... ./services/infra-fleet-service/... -count=1

# Full workspace regression check (all 20 go.work modules, not just the 2 touched):
for m in common proto cmd/orca-cli services/ai-provider-service services/annotation-service \
  services/api-gateway services/auth-service services/automation-service \
  services/credential-broker-service services/git-gateway-service services/infra-fleet-service \
  services/issue-status-sync services/issue-tracking-service services/notification-service \
  services/orchestration-service services/project-service services/scm-integration-service \
  services/task-service services/tenant-service services/usage-service services/workflow-service; do
  go build ./$m/... || echo "FAILED: $m"
done
```

Expected: clean build/vet/test everywhere, no `FAILED:` lines. TASK-019 adds the dedicated `apperrors_test.go` regression tests.
