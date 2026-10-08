package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// Deadlines of CONTRACT section 2. requestAIChannelTimeout stays under the 25s
// invokeTimeout in handler.go so a slow AI call fails with our code, not the client's.
const (
	requestRPCTimeout        = groupRPCTimeout
	requestCommitPlanTimeout = 15 * time.Second
	requestStartPhaseTimeout = 15 * time.Second
	requestAIChannelTimeout  = 24 * time.Second

	requestDefaultPageSize = 20
	requestMaxPageSize     = 100
)

// requestInputError marks a failure found by the gateway itself, relayed as is.
type requestInputError struct{ msg string }

func (e requestInputError) Error() string { return e.msg }

func invalidRequestArg(format string, a ...any) error {
	return requestInputError{msg: "INVALID_ARGUMENT: " + fmt.Sprintf(format, a...)}
}

// requestChannels carries the two downstream clients every Request channel uses.
// Either may be nil (REQUEST_SERVICE_ADDR unset): channels still register so the
// inventory is complete, and answer REQUEST_UNAVAILABLE.
type requestChannels struct {
	req  requestv1.RequestServiceClient
	appr requestv1.ApprovalServiceClient
}

type requestChannelOpts struct {
	timeout  time.Duration
	approval bool // uses the ApprovalService client and approval error codes
	ai       bool // a deadline maps to REQUEST_AI_COMPLETE_TIMEOUT
}

// handle registers one unary channel with the shared envelope: nil-client check,
// deadline, and error shaping (CONTRACT C4, C9).
func (c requestChannels) handle(r *Registry, name string, o requestChannelOpts, fn func(ctx context.Context, id Identity, args []json.RawMessage) (any, error)) {
	r.Register(name, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if (o.approval && requestClientMissing(c.appr)) || (!o.approval && requestClientMissing(c.req)) {
			return nil, errRequestUnavailable
		}
		ctx, cancel := context.WithTimeout(ctx, o.timeout)
		defer cancel()
		res, err := fn(ctx, id, args)
		if err == nil {
			return res, nil
		}
		var in requestInputError
		if errors.As(err, &in) {
			return nil, in
		}
		if o.ai && (errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded) {
			return nil, errors.New("REQUEST_AI_COMPLETE_TIMEOUT: the AI step did not finish in time; wait for the event or reload")
		}
		if o.approval {
			return nil, approvalChannelError(err)
		}
		return nil, requestChannelError(err)
	})
}

// decodeRequestArgs reads args[0]; an absent argument means the zero value, so
// parameterless channels accept {} or nothing (CONTRACT C3).
func decodeRequestArgs[T any](args []json.RawMessage) (T, error) {
	var v T
	if len(args) == 0 || len(strings.TrimSpace(string(args[0]))) == 0 || string(args[0]) == "null" {
		return v, nil
	}
	if err := json.Unmarshal(args[0], &v); err != nil {
		return v, invalidRequestArg("arg[0] must be a JSON object")
	}
	return v, nil
}

func clampRequestPageSize(n int32) int32 {
	switch {
	case n <= 0:
		return requestDefaultPageSize
	case n > requestMaxPageSize:
		return requestMaxPageSize
	}
	return n
}

var (
	requestTypeSet  = stringSet("change_request", "bug", "hotfix", "task", "spike", "question", "refactor", "security", "performance", "docs", "ops_request")
	requestStageSet = stringSet("classification", "analysis", "plan", "phase", "task")
	linkReasonSet   = stringSet("spawned_by_spike", "spawned_by_question", "followup_hotfix", "escalation")
	requestStatuses = stringSet("new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval",
		"planning", "awaiting_plan_approval", "executing", "completed", "request_backlog", "cancelled")
)

func stringSet(v ...string) map[string]bool {
	m := make(map[string]bool, len(v))
	for _, s := range v {
		m[s] = true
	}
	return m
}

// checkEnum accepts empty (unset) or a member of set.
func checkRequestEnum(field string, set map[string]bool, values ...string) error {
	for _, v := range values {
		if v != "" && !set[v] {
			return invalidRequestArg("%s has an unsupported value", field)
		}
	}
	return nil
}

// parseProtoEnum maps "request_type" to APPROVAL_SUBJECT_TYPE_REQUEST_TYPE; "" is the zero value.
func parseProtoEnum(field, prefix string, values map[string]int32, s string) (int32, error) {
	if s == "" {
		return 0, nil
	}
	n, ok := values[prefix+strings.ToUpper(s)]
	if !ok || n == 0 {
		return 0, invalidRequestArg("%s has an unsupported value", field)
	}
	return n, nil
}
