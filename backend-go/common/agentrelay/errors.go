package agentrelay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrReadonlyUnsupported means the agent refused accessMode=readonly (it would have run in write mode).
	ErrReadonlyUnsupported = errors.New("agentrelay: agent does not support readonly mode")
	// ErrKnowledgeHubUnavailable means the agent refused knowledgeRoot. The run did not start, so a caller may retry
	// without the knowledge graph.
	ErrKnowledgeHubUnavailable = errors.New("agentrelay: agent cannot load the knowledge hub")
	// ErrTimeout is context.DeadlineExceeded itself, so existing equality and errors.Is checks keep working.
	ErrTimeout = context.DeadlineExceeded
	// ErrConflictingAccess rejects a run that asks for read-only and a write profile at once.
	ErrConflictingAccess = errors.New("agentrelay: a run cannot be both read-only and a write profile")
	// ErrProfileRequired rejects a write-profile run without a profile name.
	ErrProfileRequired = errors.New("agentrelay: a write-profile run needs a profile name")
	// ErrProposeWithoutRoot rejects proposeChangesets without a knowledgeRoot, which the agent would refuse.
	ErrProposeWithoutRoot = errors.New("agentrelay: proposeChangesets needs a knowledgeRoot")
	// ErrMalformedResult marks a relay answer that is not the JSON the agent contract describes.
	ErrMalformedResult = errors.New("agentrelay: malformed result")
)

// CompleteError is a typed ai.complete refusal. infra-fleet reports it as the gRPC message "CODE: text"; the service
// that owns the user-facing wording maps Code.
type CompleteError struct {
	Code   string
	Detail string
	Cause  error
}

func (e *CompleteError) Error() string { return "agentrelay: ai.complete refused: " + e.Code }
func (e *CompleteError) Unwrap() error { return e.Cause }

const completeCodePrefix = "AI_COMPLETE_"

// isDeadline covers both a local context deadline and a deadline reported by the remote end.
func isDeadline(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded
}

// classifyExecError turns agent refusals into typed errors. Relay may flatten the JSON-RPC error data into the message,
// so the reason codes are matched in the text. Order matters: a deadline wins over any text it carries.
func classifyExecError(err error) error {
	if isDeadline(err) {
		return ErrTimeout
	}
	msg := err.Error()
	if strings.Contains(msg, "READONLY_MODE_UNSUPPORTED") {
		return ErrReadonlyUnsupported
	}
	if strings.Contains(msg, "KNOWLEDGE_HUB_UNSUPPORTED") || strings.Contains(msg, "KNOWLEDGE_ROOT_") || strings.Contains(msg, "KNOWLEDGE_CONFIG_") {
		return fmt.Errorf("%w: %w", ErrKnowledgeHubUnavailable, err)
	}
	return fmt.Errorf("agentrelay: relay %s: %w", MethodExecPrompt, err)
}

// classifyCompleteError mirrors classifyExecError for ai.complete.
func classifyCompleteError(err error) error {
	if isDeadline(err) {
		return ErrTimeout
	}
	if st, ok := status.FromError(err); ok {
		if code, detail, found := strings.Cut(st.Message(), ":"); found {
			if code = strings.TrimSpace(code); strings.HasPrefix(code, completeCodePrefix) && isUpperSnake(code) {
				return &CompleteError{Code: code, Detail: strings.TrimSpace(detail), Cause: err}
			}
		}
	}
	return fmt.Errorf("agentrelay: relay %s: %w", MethodComplete, err)
}

// isUpperSnake keeps free text that merely contains a colon from being read as a code.
func isUpperSnake(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && c != '_' && (c < '0' || c > '9') {
			return false
		}
	}
	return s != ""
}
