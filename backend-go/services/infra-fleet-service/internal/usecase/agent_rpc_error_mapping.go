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

// MapAgentExecError maps errors from Client.Exec for codeintel.* and quality.* methods
// into domain/apperrors taxonomy per contract §3.4.
func MapAgentExecError(method string, err error) error {
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
