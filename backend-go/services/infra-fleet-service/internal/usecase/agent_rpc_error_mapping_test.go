package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestMapAgentExecError_Table2B(t *testing.T) {
	cases := []struct {
		code     string
		wantKind apperrors.Kind
	}{
		{"CODEINTEL_TOOL_UNAVAILABLE", apperrors.KindFailedPrecondition},
		{"CODEINTEL_INDEX_MISSING", apperrors.KindFailedPrecondition},
		{"CODEINTEL_REPO_NOT_REGISTERED", apperrors.KindFailedPrecondition},
		{"CODEINTEL_REINDEX_IN_PROGRESS", apperrors.KindFailedPrecondition},
		{"CODEINTEL_OUTPUT_TOO_LARGE", apperrors.KindFailedPrecondition},
		{"CODEINTEL_AGENT_UNSUPPORTED", apperrors.KindFailedPrecondition},
		{"CODEINTEL_ENV_NOT_READY", apperrors.KindFailedPrecondition},
		{"CODEINTEL_RUN_IN_PROGRESS", apperrors.KindFailedPrecondition},
		{"CODEINTEL_RUN_CANCELLED", apperrors.KindFailedPrecondition},
		{"CODEINTEL_PATH_NOT_ALLOWED", apperrors.KindPermissionDenied},
		{"CODEINTEL_INVALID_PARAMS", apperrors.KindInvalidArgument},
		{"CODEINTEL_AMBIGUOUS_SYMBOL", apperrors.KindInvalidArgument},
		{"CODEINTEL_PROFILE_UNKNOWN", apperrors.KindInvalidArgument},
		{"CODEINTEL_SYMBOL_NOT_FOUND", apperrors.KindNotFound},
		{"CODEINTEL_RUN_NOT_FOUND", apperrors.KindNotFound},
		{"CODEINTEL_TIMEOUT", apperrors.KindDeadlineExceeded},
		{"CODEINTEL_TOOL_FAILED", apperrors.KindInternal},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			rawErr := &domain.AgentRPCError{
				Code:    -32000,
				Message: "agent failed for " + tc.code,
				Data:    json.RawMessage(fmt.Sprintf(`{"code":%q}`, tc.code)),
			}

			mapped := MapAgentExecError("codeintel.overview", rawErr)
			var appErr *apperrors.AppError
			if !errors.As(mapped, &appErr) {
				t.Fatalf("expected *apperrors.AppError, got %T (%v)", mapped, mapped)
			}

			if appErr.Kind != tc.wantKind {
				t.Errorf("kind mismatch: got %v, want %v", appErr.Kind, tc.wantKind)
			}
			if appErr.Code != tc.code {
				t.Errorf("code mismatch: got %s, want %s", appErr.Code, tc.code)
			}
		})
	}
}

func TestMapAgentExecError_MethodNotFound(t *testing.T) {
	err := MapAgentExecError("quality.run", domain.ErrAgentMethodNotFound)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindFailedPrecondition {
		t.Errorf("expected KindFailedPrecondition, got %v", appErr.Kind)
	}
	if appErr.Code != "CODEINTEL_AGENT_UNSUPPORTED" {
		t.Errorf("expected CODEINTEL_AGENT_UNSUPPORTED, got %s", appErr.Code)
	}
}

func TestMapAgentExecError_DeadlineExceeded(t *testing.T) {
	err := MapAgentExecError("codeintel.symbol", context.DeadlineExceeded)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindDeadlineExceeded {
		t.Errorf("expected KindDeadlineExceeded, got %v", appErr.Kind)
	}
	if appErr.Code != "CODEINTEL_TIMEOUT" {
		t.Errorf("expected CODEINTEL_TIMEOUT, got %s", appErr.Code)
	}
}

func TestMapAgentExecError_NonCodeIntelMethod_LegacyBehavior(t *testing.T) {
	rawErr := &domain.AgentRPCError{
		Code:    -32000,
		Message: "ports error",
		Data:    json.RawMessage(`{"code":"CODEINTEL_INDEX_MISSING"}`),
	}

	err := MapAgentExecError("ports.scan", rawErr)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.AppError, got %T", err)
	}
	if appErr.Code != "INFRA_AGENT_EXEC_FAILED" {
		t.Errorf("expected INFRA_AGENT_EXEC_FAILED for non-codeintel method, got %s", appErr.Code)
	}
}

func TestMapAgentExecError_UnknownCode_FallsBackToInfraAgentExecFailed(t *testing.T) {
	rawErr := &domain.AgentRPCError{
		Code:    -32000,
		Message: "unknown code error",
		Data:    json.RawMessage(`{"code":"CODEINTEL_UNKNOWN_CODE_XYZ"}`),
	}

	err := MapAgentExecError("codeintel.overview", rawErr)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.AppError, got %T", err)
	}
	if appErr.Code != "INFRA_AGENT_EXEC_FAILED" {
		t.Errorf("expected INFRA_AGENT_EXEC_FAILED, got %s", appErr.Code)
	}
}

func TestAgentErrorDataForTrailer_ShrinksLargeData(t *testing.T) {
	// Create payload with 12 large candidates exceeding 4 KiB
	candidates := make([]map[string]any, 12)
	for i := range candidates {
		candidates[i] = map[string]any{
			"key":       fmt.Sprintf("method:src/services/very/long/path/module_%d.ts:Service.method%d", i, i),
			"name":      fmt.Sprintf("method%d", i),
			"signature": fmt.Sprintf("(arg1: string, arg2: number, arg3: boolean) => Promise<Result%d>", i),
			"doc":       strings.Repeat("A very long docstring explaining the symbol details. ", 20),
		}
	}
	payload := map[string]any{
		"code":       "CODEINTEL_AMBIGUOUS_SYMBOL",
		"candidates": candidates,
		"extra":      strings.Repeat("extra details ", 50),
	}
	data, _ := json.Marshal(payload)
	if len(data) <= 4096 {
		t.Fatalf("test payload should exceed 4096 bytes, but got %d", len(data))
	}

	trailerBytes := AgentErrorDataForTrailer(data, 4096)
	if len(trailerBytes) > 4096 {
		t.Errorf("expected trailerBytes <= 4096, got %d", len(trailerBytes))
	}

	// Must be valid JSON and retain code
	var decoded map[string]any
	if err := json.Unmarshal(trailerBytes, &decoded); err != nil {
		t.Fatalf("trailer bytes must be valid JSON: %v", err)
	}
	if decoded["code"] != "CODEINTEL_AMBIGUOUS_SYMBOL" {
		t.Errorf("expected code preserved, got %v", decoded["code"])
	}
}
