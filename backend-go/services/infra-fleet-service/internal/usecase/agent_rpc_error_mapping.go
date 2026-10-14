package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// agentPathNotFoundCode is AgentErrorCode.PathNotFound of the agent's wire protocol (agent/src/shared/agent-wire-protocol.ts).
const agentPathNotFoundCode = -33003

var allowedAgentErrorCodes = map[string]apperrors.Kind{
	"CODEINTEL_TOOL_UNAVAILABLE":    apperrors.KindFailedPrecondition,
	"CODEINTEL_INDEX_MISSING":       apperrors.KindFailedPrecondition,
	"CODEINTEL_REPO_NOT_REGISTERED": apperrors.KindFailedPrecondition,
	"CODEINTEL_REINDEX_IN_PROGRESS": apperrors.KindFailedPrecondition,
	"CODEINTEL_OUTPUT_TOO_LARGE":    apperrors.KindFailedPrecondition,
	"CODEINTEL_AGENT_UNSUPPORTED":   apperrors.KindFailedPrecondition,
	"CODEINTEL_ENV_NOT_READY":       apperrors.KindFailedPrecondition,
	"CODEINTEL_RUN_IN_PROGRESS":     apperrors.KindFailedPrecondition,
	"CODEINTEL_RUN_CANCELLED":       apperrors.KindFailedPrecondition,
	"CODEINTEL_PATH_NOT_ALLOWED":    apperrors.KindPermissionDenied,
	"CODEINTEL_INVALID_PARAMS":      apperrors.KindInvalidArgument,
	"CODEINTEL_AMBIGUOUS_SYMBOL":    apperrors.KindInvalidArgument,
	"CODEINTEL_PROFILE_UNKNOWN":     apperrors.KindInvalidArgument,
	"CODEINTEL_SYMBOL_NOT_FOUND":    apperrors.KindNotFound,
	"CODEINTEL_RUN_NOT_FOUND":       apperrors.KindNotFound,
	"CODEINTEL_TIMEOUT":             apperrors.KindDeadlineExceeded,
	"CODEINTEL_TOOL_FAILED":         apperrors.KindInternal,
}

// aiCompleteReasons maps the agent's ai.complete error.data.reason to a typed error the caller can act on:
// "log in on the dev server" and "quota exhausted" need different fixes than "try again".
var aiCompleteReasons = map[string]struct {
	kind apperrors.Kind
	code string
}{
	"NO_API_KEY":             {apperrors.KindFailedPrecondition, "AI_COMPLETE_NOT_CONFIGURED"},
	"CLI_NOT_LOGGED_IN":      {apperrors.KindFailedPrecondition, "AI_COMPLETE_CLI_NOT_LOGGED_IN"},
	"CLI_SESSION_EXPIRED":    {apperrors.KindFailedPrecondition, "AI_COMPLETE_SESSION_EXPIRED"},
	"CLI_UNAVAILABLE":        {apperrors.KindFailedPrecondition, "AI_COMPLETE_CLI_UNAVAILABLE"},
	"CLI_QUOTA_EXHAUSTED":    {apperrors.KindResourceExhausted, "AI_COMPLETE_QUOTA_EXHAUSTED"},
	"CLI_BUSY":               {apperrors.KindUnavailable, "AI_COMPLETE_BUSY"},
	"CLI_FAILED":             {apperrors.KindInternal, "AI_COMPLETE_FAILED"},
	"PROVIDER_TIMEOUT":       {apperrors.KindDeadlineExceeded, "AI_COMPLETE_TIMEOUT"},
	"PROVIDER_NETWORK":       {apperrors.KindUnavailable, "AI_COMPLETE_PROVIDER_UNAVAILABLE"},
	"PROVIDER_HTTP_ERROR":    {apperrors.KindUnavailable, "AI_COMPLETE_PROVIDER_UNAVAILABLE"},
	"UNKNOWN_MODEL_PROVIDER": {apperrors.KindInvalidArgument, "AI_COMPLETE_UNKNOWN_MODEL"},
}

func mapAIExecError(err error) error {
	var rpcErr *domain.AgentRPCError
	if errors.As(err, &rpcErr) {
		var payload struct {
			Reason string `json:"reason"`
		}
		if len(rpcErr.Data) > 0 && json.Unmarshal(rpcErr.Data, &payload) == nil {
			if m, ok := aiCompleteReasons[payload.Reason]; ok {
				return apperrors.New(m.kind, m.code, sanitizeMessage(rpcErr.Message, 300), err)
			}
		}
	}
	if errors.Is(err, domain.ErrAgentMethodNotFound) {
		return apperrors.New(apperrors.KindFailedPrecondition, "AI_COMPLETE_UNSUPPORTED", "this dev server agent does not support ai.complete", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.New(apperrors.KindDeadlineExceeded, "AI_COMPLETE_TIMEOUT", "ai.complete timed out", err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)
}

// MapAgentExecError maps errors from Client.Exec into domain/apperrors taxonomy: codeintel.* and quality.* per
// contract §3.4, and ai.complete by its structured reason.
func MapAgentExecError(method string, err error) error {
	if method == "ai.complete" {
		return mapAIExecError(err)
	}
	if strings.HasPrefix(method, "fs.") {
		var rpcErr *domain.AgentRPCError
		if errors.As(err, &rpcErr) && rpcErr.Code == agentPathNotFoundCode {
			// A missing path is an answer, not a relay failure: callers such as git-gateway's StatFile turn it into exists=false.
			return apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_PATH_NOT_FOUND", "path not found on the dev server", err)
		}
	}
	if !strings.HasPrefix(method, "codeintel.") && !strings.HasPrefix(method, "quality.") {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)
	}

	if errors.Is(err, domain.ErrAgentMethodNotFound) {
		return apperrors.New(apperrors.KindFailedPrecondition, "CODEINTEL_AGENT_UNSUPPORTED", "method not supported by agent", err)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return apperrors.New(apperrors.KindDeadlineExceeded, "CODEINTEL_TIMEOUT", "agent exec timed out", err)
	}

	var rpcErr *domain.AgentRPCError
	if errors.As(err, &rpcErr) {
		var payload struct {
			Code string `json:"code"`
		}
		if len(rpcErr.Data) > 0 && json.Unmarshal(rpcErr.Data, &payload) == nil {
			if kind, ok := allowedAgentErrorCodes[payload.Code]; ok {
				sanitized := sanitizeMessage(rpcErr.Message, 300)
				return apperrors.New(kind, payload.Code, sanitized, err)
			}
		}
	}

	return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)
}

func sanitizeMessage(s string, maxRunes int) string {
	var sb strings.Builder
	runes := 0
	for _, r := range s {
		if runes >= maxRunes {
			break
		}
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		sb.WriteRune(r)
		runes++
	}
	return sb.String()
}

// AgentErrorDataForTrailer formats and shrinks agent error data payload to fit within the byte limit.
// It prioritizes preserving the "code" field, trimming large arrays first, then non-code keys.
func AgentErrorDataForTrailer(data json.RawMessage, limit int) []byte {
	if len(data) <= limit {
		return data
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}

	for {
		encoded, err := json.Marshal(obj)
		if err == nil && len(encoded) <= limit {
			return encoded
		}

		// Find the longest slice to prune
		var longestKey string
		var maxLen int
		for k, v := range obj {
			if slice, ok := v.([]any); ok && len(slice) > maxLen {
				longestKey = k
				maxLen = len(slice)
			}
		}

		if longestKey != "" && maxLen > 0 {
			slice := obj[longestKey].([]any)
			obj[longestKey] = slice[:len(slice)-1]
			continue
		}

		// Otherwise, delete keys other than "code" in deterministic sorted order
		keys := make([]string, 0, len(obj))
		for k := range obj {
			if k != "code" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		if len(keys) == 0 {
			// Only code remains (or nothing left)
			encoded, _ = json.Marshal(obj)
			return encoded
		}

		delete(obj, keys[len(keys)-1])
	}
}
