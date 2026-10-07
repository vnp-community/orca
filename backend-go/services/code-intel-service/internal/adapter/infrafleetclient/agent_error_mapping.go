package infrafleetclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
)

const maxTrailerDataBytes = 4096

// AgentError wraps an AppError with structured error data decoded from agent trailers.
type AgentError struct {
	*apperrors.AppError
	Data json.RawMessage
}

func (e *AgentError) Unwrap() error {
	return e.AppError
}

// GetAgentErrorData extracts the structured agent error data from err, if present.
func GetAgentErrorData(err error) (json.RawMessage, bool) {
	var ae *AgentError
	if errors.As(err, &ae) && len(ae.Data) > 0 {
		return ae.Data, true
	}
	return nil, false
}

var codeIntelPrefixRegex = regexp.MustCompile(`^(CODEINTEL_[A-Z0-9_]+|INFRA_[A-Z0-9_]+):\s*(.*)$`)

// sanitizeMessage trims msg to maxLen and removes control characters.
func sanitizeMessage(msg string, maxLen int) string {
	var b strings.Builder
	for _, r := range msg {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// MapRelayError translates gRPC errors and agent JSON-RPC status codes into apperrors per Table 2.C.
func MapRelayError(err error, trailer metadata.MD) error {
	if err == nil {
		return nil
	}

	// 1. Read trailer x-orca-agent-error-data-bin (<= 4 KiB)
	var agentData json.RawMessage
	if trailer != nil {
		trailers := trailer.Get("x-orca-agent-error-data-bin")
		if len(trailers) > 0 && len(trailers[0]) > 0 && len(trailers[0]) <= maxTrailerDataBytes {
			var raw json.RawMessage
			if json.Unmarshal([]byte(trailers[0]), &raw) == nil {
				agentData = raw
			}
		}
	}

	st, ok := status.FromError(err)
	if !ok {
		appErr := apperrors.New(apperrors.KindInternal, "CODEINTEL_TOOL_FAILED", sanitizeMessage(err.Error(), 200), err)
		if len(agentData) > 0 {
			return &AgentError{AppError: appErr, Data: agentData}
		}
		return appErr
	}

	// Check gRPC wire code first for transport-level issues
	switch st.Code() {
	case codes.DeadlineExceeded:
		appErr := apperrors.New(apperrors.KindDeadlineExceeded, "CODEINTEL_TIMEOUT", "agent call timed out", err)
		if len(agentData) > 0 {
			return &AgentError{AppError: appErr, Data: agentData}
		}
		return appErr
	case codes.Unavailable:
		appErr := apperrors.New(apperrors.KindUnavailable, "CODEINTEL_DEV_SERVER_OFFLINE", "dev server is offline or unavailable", err)
		if len(agentData) > 0 {
			return &AgentError{AppError: appErr, Data: agentData}
		}
		return appErr
	case codes.ResourceExhausted:
		appErr := apperrors.New(apperrors.KindFailedPrecondition, "CODEINTEL_OUTPUT_TOO_LARGE", "agent output exceeded maximum allowed size", err)
		if len(agentData) > 0 {
			return &AgentError{AppError: appErr, Data: agentData}
		}
		return appErr
	}

	// Parse message code prefix
	rawMsg := st.Message()
	matches := codeIntelPrefixRegex.FindStringSubmatch(rawMsg)
	if len(matches) == 3 {
		code := matches[1]
		msg := matches[2]
		if msg == "" {
			msg = code
		}

		var kind apperrors.Kind
		switch code {
		// Infra-fleet specific codes
		case "INFRA_DEV_SERVER_NOT_CONNECTED":
			code = "CODEINTEL_DEV_SERVER_OFFLINE"
			kind = apperrors.KindUnavailable
		case "INFRA_DEV_SERVER_NOT_FOUND":
			code = "CODEINTEL_REPO_NOT_REGISTERED"
			kind = apperrors.KindNotFound
		case "INFRA_NO_TENANT":
			kind = apperrors.KindUnauthenticated

		// CodeIntel specific codes
		case "CODEINTEL_TOOL_UNAVAILABLE",
			"CODEINTEL_INDEX_MISSING",
			"CODEINTEL_REPO_NOT_REGISTERED",
			"CODEINTEL_REINDEX_IN_PROGRESS",
			"CODEINTEL_OUTPUT_TOO_LARGE",
			"CODEINTEL_AGENT_UNSUPPORTED",
			"CODEINTEL_ENV_NOT_READY",
			"CODEINTEL_RUN_IN_PROGRESS",
			"CODEINTEL_RUN_CANCELLED":
			kind = apperrors.KindFailedPrecondition

		case "CODEINTEL_PATH_NOT_ALLOWED":
			kind = apperrors.KindPermissionDenied
			// Hook for audit logging when SOL-013 is wired

		case "CODEINTEL_INVALID_PARAMS",
			"CODEINTEL_AMBIGUOUS_SYMBOL",
			"CODEINTEL_PROFILE_UNKNOWN":
			kind = apperrors.KindInvalidArgument

		case "CODEINTEL_SYMBOL_NOT_FOUND",
			"CODEINTEL_RUN_NOT_FOUND":
			kind = apperrors.KindNotFound

		case "CODEINTEL_TIMEOUT":
			kind = apperrors.KindDeadlineExceeded

		case "CODEINTEL_TOOL_FAILED":
			kind = apperrors.KindInternal

		default:
			// Unknown code: fallback to CODEINTEL_TOOL_FAILED, truncate msg <= 200, strip control chars
			code = "CODEINTEL_TOOL_FAILED"
			kind = apperrors.KindInternal
			msg = sanitizeMessage(fmt.Sprintf("%s: %s", matches[1], matches[2]), 200)
		}

		appErr := apperrors.New(kind, code, msg, err)
		if len(agentData) > 0 {
			return &AgentError{AppError: appErr, Data: agentData}
		}
		return appErr
	}

	// Fallback for unmapped errors
	appErr := apperrors.New(apperrors.KindInternal, "CODEINTEL_TOOL_FAILED", sanitizeMessage(rawMsg, 200), err)
	if len(agentData) > 0 {
		return &AgentError{AppError: appErr, Data: agentData}
	}
	return appErr
}
