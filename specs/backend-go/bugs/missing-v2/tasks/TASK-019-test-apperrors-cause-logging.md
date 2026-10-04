# TASK-019: Tests for `apperrors.ToGRPCStatus`'s optional cause-logging

**From Solution:** SOL-009
**Priority:** Low
**Service:** `common/apperrors`
**File:** `common/apperrors/apperrors_test.go` (new — confirmed no test file existed in this package before this task)
**Depends on:** TASK-018
**Status:** `[x]` DONE — added `TestToGRPCStatus_NoLoggerWired_Unaffected`, `TestToGRPCStatus_LoggerWired_LogsCauseServerSideOnly`, `TestToGRPCStatus_NoWrappedCause_NeverLogs`. All pass; `go test ./common/apperrors/... -count=1 -v` clean.

---

## Context

`common/apperrors` had zero test coverage before this task. TASK-018's change is small but touches a CRITICAL-blast-radius shared function (463 direct callers) — needs a direct regression guard that the no-logger-wired path is byte-for-byte unaffected, not just "it compiles."

## Changes made

`common/apperrors/apperrors_test.go`:

```go
func TestToGRPCStatus_NoLoggerWired_Unaffected(t *testing.T) {
	logger = nil // isolate from any other test in this package setting it
	cause := errors.New("devserveragent: not connected")
	err := ToGRPCStatus(New(KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", cause))

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected codes.Internal, got %v", st.Code())
	}
	if st.Message() != "INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent" {
		t.Errorf("expected the client-safe message unchanged (no cause leaked), got %q", st.Message())
	}
}

func TestToGRPCStatus_LoggerWired_LogsCauseServerSideOnly(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { logger = nil })

	cause := errors.New("devserveragent: not connected")
	err := ToGRPCStatus(New(KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", cause))

	st, _ := status.FromError(err)
	if strings.Contains(st.Message(), "not connected") {
		t.Error("the wrapped cause must never appear in the client-visible status message")
	}
	logged := buf.String()
	if !strings.Contains(logged, "INFRA_AGENT_EXEC_FAILED") || !strings.Contains(logged, "devserveragent: not connected") {
		t.Errorf("expected the server-side log to contain the code and cause, got %q", logged)
	}
}

func TestToGRPCStatus_NoWrappedCause_NeverLogs(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { logger = nil })

	_ = ToGRPCStatus(New(KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", nil))

	if buf.Len() != 0 {
		t.Errorf("expected no log output when AppError.Err is nil, got %q", buf.String())
	}
}
```

Each test resets the package-level `logger` var via `t.Cleanup`/direct assignment so tests in this package never leak state into each other — there's no exported reset function since `SetLogger(nil)` is a valid, supported way to unwire it (matches `SetAuditClient`'s own convention in `project-service`).

## Verify

```bash
cd backend-go
go test ./common/apperrors/... -count=1 -v
```

Expected: all 3 tests pass.
