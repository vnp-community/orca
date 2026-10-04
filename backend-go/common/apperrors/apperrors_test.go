package apperrors

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestToGRPCStatus_NoLoggerWired_Unaffected is the regression test for
// BUG-009's fix: SetLogger is opt-in — a service that never calls it (i.e.
// every existing caller before this change) must see byte-for-byte the same
// returned status as before.
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

// TestToGRPCStatus_LoggerWired_LogsCauseServerSideOnly is BUG-009's positive
// case: once SetLogger is wired, the wrapped cause reaches this service's
// own structured logs — but the client-visible status is still unchanged.
func TestToGRPCStatus_LoggerWired_LogsCauseServerSideOnly(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { logger = nil })

	cause := errors.New("devserveragent: not connected")
	err := ToGRPCStatus(New(KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", cause))

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Message() != "INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent" {
		t.Errorf("expected the client-safe message unchanged even with a logger wired, got %q", st.Message())
	}
	if strings.Contains(st.Message(), "not connected") {
		t.Error("the wrapped cause must never appear in the client-visible status message")
	}

	logged := buf.String()
	if !strings.Contains(logged, "INFRA_AGENT_EXEC_FAILED") || !strings.Contains(logged, "devserveragent: not connected") {
		t.Errorf("expected the server-side log to contain the code and cause, got %q", logged)
	}
}

// TestToGRPCStatus_NoWrappedCause_NeverLogs guards against logging a no-op
// line for the (common) case of an AppError with no wrapped Err at all.
func TestToGRPCStatus_NoWrappedCause_NeverLogs(t *testing.T) {
	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { logger = nil })

	_ = ToGRPCStatus(New(KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", nil))

	if buf.Len() != 0 {
		t.Errorf("expected no log output when AppError.Err is nil, got %q", buf.String())
	}
}
