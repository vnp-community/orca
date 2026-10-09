package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func aiAgentErr(reason, message string) error {
	data, _ := json.Marshal(map[string]any{"provider": "claude_cli", "httpStatus": nil, "retryable": false, "reason": reason})
	return &domain.AgentRPCError{Code: -32000, Message: message, Data: data}
}

func TestMapAgentExecError_AIComplete(t *testing.T) {
	cases := []struct {
		reason   string
		wantKind apperrors.Kind
		wantCode string
	}{
		{"NO_API_KEY", apperrors.KindFailedPrecondition, "AI_COMPLETE_NOT_CONFIGURED"},
		{"CLI_NOT_LOGGED_IN", apperrors.KindFailedPrecondition, "AI_COMPLETE_CLI_NOT_LOGGED_IN"},
		{"CLI_SESSION_EXPIRED", apperrors.KindFailedPrecondition, "AI_COMPLETE_SESSION_EXPIRED"},
		{"CLI_UNAVAILABLE", apperrors.KindFailedPrecondition, "AI_COMPLETE_CLI_UNAVAILABLE"},
		{"CLI_QUOTA_EXHAUSTED", apperrors.KindResourceExhausted, "AI_COMPLETE_QUOTA_EXHAUSTED"},
		{"CLI_BUSY", apperrors.KindUnavailable, "AI_COMPLETE_BUSY"},
		{"CLI_FAILED", apperrors.KindInternal, "AI_COMPLETE_FAILED"},
		{"PROVIDER_TIMEOUT", apperrors.KindDeadlineExceeded, "AI_COMPLETE_TIMEOUT"},
		{"PROVIDER_NETWORK", apperrors.KindUnavailable, "AI_COMPLETE_PROVIDER_UNAVAILABLE"},
		{"PROVIDER_HTTP_ERROR", apperrors.KindUnavailable, "AI_COMPLETE_PROVIDER_UNAVAILABLE"},
		{"UNKNOWN_MODEL_PROVIDER", apperrors.KindInvalidArgument, "AI_COMPLETE_UNKNOWN_MODEL"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			got := MapAgentExecError("ai.complete", aiAgentErr(tc.reason, "agent said so"))
			var ae *apperrors.AppError
			if !errors.As(got, &ae) || ae.Kind != tc.wantKind || ae.Code != tc.wantCode {
				t.Fatalf("got %v, want %v/%s", got, tc.wantKind, tc.wantCode)
			}
		})
	}
}

func TestMapAgentExecError_AICompleteFallbacks(t *testing.T) {
	var ae *apperrors.AppError
	if got := MapAgentExecError("ai.complete", domain.ErrAgentMethodNotFound); !errors.As(got, &ae) || ae.Code != "AI_COMPLETE_UNSUPPORTED" {
		t.Fatalf("method not found: %v", got)
	}
	if got := MapAgentExecError("ai.complete", context.DeadlineExceeded); !errors.As(got, &ae) || ae.Code != "AI_COMPLETE_TIMEOUT" {
		t.Fatalf("deadline: %v", got)
	}
	// An unknown reason must not leak through as a typed code.
	if got := MapAgentExecError("ai.complete", aiAgentErr("SOMETHING_NEW", "x")); !errors.As(got, &ae) || ae.Code != "INFRA_AGENT_EXEC_FAILED" {
		t.Fatalf("unknown reason: %v", got)
	}
	// Other methods keep the generic error.
	if got := MapAgentExecError("fs.stat", errors.New("boom")); !errors.As(got, &ae) || ae.Code != "INFRA_AGENT_EXEC_FAILED" {
		t.Fatalf("other method: %v", got)
	}
}
