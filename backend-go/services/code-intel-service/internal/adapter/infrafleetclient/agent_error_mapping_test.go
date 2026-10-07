package infrafleetclient

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestErrorMapping_Table2C(t *testing.T) {
	testCases := []struct {
		name         string
		inErr        error
		trailer      metadata.MD
		wantKind     apperrors.Kind
		wantCode     string
		checkDataKey string
	}{
		// 1. FailedPrecondition group
		{
			name:     "ToolUnavailable",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_TOOL_UNAVAILABLE: tool not installed"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_TOOL_UNAVAILABLE",
		},
		{
			name:     "IndexMissing",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_INDEX_MISSING: index missing"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_INDEX_MISSING",
		},
		{
			name:     "RepoNotRegistered",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_REPO_NOT_REGISTERED: not registered"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_REPO_NOT_REGISTERED",
		},
		{
			name:     "ReindexInProgress",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_REINDEX_IN_PROGRESS: active job"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_REINDEX_IN_PROGRESS",
		},
		{
			name:     "OutputTooLarge",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_OUTPUT_TOO_LARGE: exceeded size"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_OUTPUT_TOO_LARGE",
		},
		{
			name:     "AgentUnsupported",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_AGENT_UNSUPPORTED: not implemented"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_AGENT_UNSUPPORTED",
		},
		{
			name:     "EnvNotReady",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_ENV_NOT_READY: env down"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_ENV_NOT_READY",
		},
		{
			name:     "RunInProgress",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_RUN_IN_PROGRESS: active run"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_RUN_IN_PROGRESS",
		},
		{
			name:     "RunCancelled",
			inErr:    status.Error(codes.FailedPrecondition, "CODEINTEL_RUN_CANCELLED: cancelled"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_RUN_CANCELLED",
		},

		// 2. PermissionDenied group
		{
			name:     "PathNotAllowed",
			inErr:    status.Error(codes.PermissionDenied, "CODEINTEL_PATH_NOT_ALLOWED: path outside repo"),
			wantKind: apperrors.KindPermissionDenied,
			wantCode: "CODEINTEL_PATH_NOT_ALLOWED",
		},

		// 3. InvalidArgument group
		{
			name:     "InvalidParams",
			inErr:    status.Error(codes.InvalidArgument, "CODEINTEL_INVALID_PARAMS: bad depth"),
			wantKind: apperrors.KindInvalidArgument,
			wantCode: "CODEINTEL_INVALID_PARAMS",
		},
		{
			name:         "AmbiguousSymbolWithCandidates",
			inErr:        status.Error(codes.InvalidArgument, "CODEINTEL_AMBIGUOUS_SYMBOL: multiple candidates found"),
			trailer:      metadata.Pairs("x-orca-agent-error-data-bin", `{"candidates":[{"name":"foo","file":"a.go"},{"name":"foo","file":"b.go"}]}`),
			wantKind:     apperrors.KindInvalidArgument,
			wantCode:     "CODEINTEL_AMBIGUOUS_SYMBOL",
			checkDataKey: "candidates",
		},
		{
			name:     "ProfileUnknown",
			inErr:    status.Error(codes.InvalidArgument, "CODEINTEL_PROFILE_UNKNOWN: unknown profile"),
			wantKind: apperrors.KindInvalidArgument,
			wantCode: "CODEINTEL_PROFILE_UNKNOWN",
		},

		// 4. NotFound group
		{
			name:     "SymbolNotFound",
			inErr:    status.Error(codes.NotFound, "CODEINTEL_SYMBOL_NOT_FOUND: symbol not found"),
			wantKind: apperrors.KindNotFound,
			wantCode: "CODEINTEL_SYMBOL_NOT_FOUND",
		},
		{
			name:     "RunNotFound",
			inErr:    status.Error(codes.NotFound, "CODEINTEL_RUN_NOT_FOUND: run not found"),
			wantKind: apperrors.KindNotFound,
			wantCode: "CODEINTEL_RUN_NOT_FOUND",
		},

		// 5. DeadlineExceeded group
		{
			name:     "Timeout",
			inErr:    status.Error(codes.DeadlineExceeded, "CODEINTEL_TIMEOUT: timed out"),
			wantKind: apperrors.KindDeadlineExceeded,
			wantCode: "CODEINTEL_TIMEOUT",
		},
		{
			name:     "GRPCCodeDeadlineExceeded",
			inErr:    status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
			wantKind: apperrors.KindDeadlineExceeded,
			wantCode: "CODEINTEL_TIMEOUT",
		},

		// 6. Internal group
		{
			name:     "ToolFailed",
			inErr:    status.Error(codes.Internal, "CODEINTEL_TOOL_FAILED: command exited with 1"),
			wantKind: apperrors.KindInternal,
			wantCode: "CODEINTEL_TOOL_FAILED",
		},

		// 7. Infra-fleet errors
		{
			name:     "InfraDevServerNotConnected",
			inErr:    status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: no live connection"),
			wantKind: apperrors.KindUnavailable,
			wantCode: "CODEINTEL_DEV_SERVER_OFFLINE",
		},
		{
			name:     "InfraDevServerNotFound",
			inErr:    status.Error(codes.NotFound, "INFRA_DEV_SERVER_NOT_FOUND: dev server not found"),
			wantKind: apperrors.KindNotFound,
			wantCode: "CODEINTEL_REPO_NOT_REGISTERED",
		},
		{
			name:     "InfraNoTenant",
			inErr:    status.Error(codes.Unauthenticated, "INFRA_NO_TENANT: no tenant in context"),
			wantKind: apperrors.KindUnauthenticated,
			wantCode: "INFRA_NO_TENANT",
		},

		// 8. gRPC wire codes
		{
			name:     "GRPCUnavailable",
			inErr:    status.Error(codes.Unavailable, "connection refused"),
			wantKind: apperrors.KindUnavailable,
			wantCode: "CODEINTEL_DEV_SERVER_OFFLINE",
		},
		{
			name:     "GRPCResourceExhausted",
			inErr:    status.Error(codes.ResourceExhausted, "message larger than max"),
			wantKind: apperrors.KindFailedPrecondition,
			wantCode: "CODEINTEL_OUTPUT_TOO_LARGE",
		},

		// 9. Unknown/unmapped error
		{
			name:     "UnknownErrorCodeFallback",
			inErr:    status.Error(codes.Unknown, "SOMETHING_STRANGE: mysterious failure"),
			wantKind: apperrors.KindInternal,
			wantCode: "CODEINTEL_TOOL_FAILED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := MapRelayError(tc.inErr, tc.trailer)
			if mapped == nil {
				t.Fatal("expected mapped error, got nil")
			}

			var ae *apperrors.AppError
			if !errors.As(mapped, &ae) {
				t.Fatalf("mapped error does not unwrap to *apperrors.AppError: %T", mapped)
			}

			if ae.Kind != tc.wantKind {
				t.Errorf("ae.Kind = %v, want %v", ae.Kind, tc.wantKind)
			}
			if ae.Code != tc.wantCode {
				t.Errorf("ae.Code = %q, want %q", ae.Code, tc.wantCode)
			}

			// Ensure CODEINTEL_INVALID_ARGUMENT is never emitted
			if ae.Code == "CODEINTEL_INVALID_ARGUMENT" || ae.Code == "INVALID_ARGUMENT" {
				t.Errorf("prohibited error code emitted: %q", ae.Code)
			}

			if tc.checkDataKey != "" {
				data, ok := GetAgentErrorData(mapped)
				if !ok || len(data) == 0 {
					t.Fatalf("expected agent error data for %s", tc.name)
				}
				var m map[string]any
				if err := json.Unmarshal(data, &m); err != nil {
					t.Fatalf("unmarshaling agent data: %v", err)
				}
				if _, found := m[tc.checkDataKey]; !found {
					t.Errorf("key %q not found in agent data: %s", tc.checkDataKey, string(data))
				}
			}
		})
	}
}

func TestErrorMapping_TrailerCorruptedGracefullyIgnored(t *testing.T) {
	// Trailer with invalid JSON
	badJSONTrailer := metadata.Pairs("x-orca-agent-error-data-bin", "{invalid json")
	mapped := MapRelayError(status.Error(codes.InvalidArgument, "CODEINTEL_INVALID_PARAMS: bad value"), badJSONTrailer)
	var ae *apperrors.AppError
	if !errors.As(mapped, &ae) || ae.Code != "CODEINTEL_INVALID_PARAMS" {
		t.Errorf("expected CODEINTEL_INVALID_PARAMS, got %v", mapped)
	}
	if _, ok := GetAgentErrorData(mapped); ok {
		t.Error("expected no agent error data for corrupted json")
	}

	// Trailer larger than 4 KiB
	oversizedTrailer := metadata.Pairs("x-orca-agent-error-data-bin", strings.Repeat("x", 4097))
	mapped2 := MapRelayError(status.Error(codes.InvalidArgument, "CODEINTEL_INVALID_PARAMS: bad value"), oversizedTrailer)
	if !errors.As(mapped2, &ae) || ae.Code != "CODEINTEL_INVALID_PARAMS" {
		t.Errorf("expected CODEINTEL_INVALID_PARAMS, got %v", mapped2)
	}
	if _, ok := GetAgentErrorData(mapped2); ok {
		t.Error("expected no agent error data for oversized trailer")
	}
}

func TestErrorMapping_SanitizeMessage(t *testing.T) {
	longMsg := strings.Repeat("A", 300)
	msgWithControl := "Hello\x00\x01\x1bWorld\n"
	sanitizedLong := sanitizeMessage(longMsg, 200)
	if len(sanitizedLong) != 200 {
		t.Errorf("len(sanitizedLong) = %d, want 200", len(sanitizedLong))
	}

	sanitizedCtrl := sanitizeMessage(msgWithControl, 200)
	if strings.ContainsAny(sanitizedCtrl, "\x00\x01\x1b") {
		t.Errorf("control chars not stripped: %q", sanitizedCtrl)
	}
}
