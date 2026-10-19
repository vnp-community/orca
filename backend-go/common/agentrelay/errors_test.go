package agentrelay

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClassifyExecError(t *testing.T) {
	plain := errors.New("connection reset")
	tests := []struct {
		name   string
		err    error
		is     error // the sentinel errors.Is must find
		isNot  []error
		bareIs bool // the error must be exactly the sentinel
	}{
		{"local deadline", context.DeadlineExceeded, ErrTimeout, nil, true},
		{"wrapped local deadline", fmt.Errorf("dial: %w", context.DeadlineExceeded), ErrTimeout, nil, true},
		{"remote deadline", status.Error(codes.DeadlineExceeded, "agent took too long"), ErrTimeout, nil, true},
		{"deadline beats refusal text", status.Error(codes.DeadlineExceeded, "READONLY_MODE_UNSUPPORTED"), ErrTimeout, []error{ErrReadonlyUnsupported}, true},
		{"read-only refused", status.Error(codes.FailedPrecondition, "agent.execPrompt: READONLY_MODE_UNSUPPORTED (FLAG_TOOLS_MISSING)"), ErrReadonlyUnsupported, []error{ErrKnowledgeHubUnavailable}, true},
		{"knowledge hub unsupported", status.Error(codes.FailedPrecondition, "KNOWLEDGE_HUB_UNSUPPORTED (no mcp flag)"), ErrKnowledgeHubUnavailable, []error{ErrReadonlyUnsupported}, false},
		{"knowledge root missing", status.Error(codes.InvalidArgument, "KNOWLEDGE_ROOT_MISSING"), ErrKnowledgeHubUnavailable, nil, false},
		{"knowledge root not absolute", status.Error(codes.InvalidArgument, "KNOWLEDGE_ROOT_NOT_ABSOLUTE"), ErrKnowledgeHubUnavailable, nil, false},
		{"knowledge root not a directory", status.Error(codes.InvalidArgument, "KNOWLEDGE_ROOT_NOT_DIRECTORY"), ErrKnowledgeHubUnavailable, nil, false},
		{"knowledge config missing", status.Error(codes.FailedPrecondition, "KNOWLEDGE_CONFIG_MISSING"), ErrKnowledgeHubUnavailable, nil, false},
		{"other knowledge config code", status.Error(codes.FailedPrecondition, "KNOWLEDGE_CONFIG_INVALID"), ErrKnowledgeHubUnavailable, nil, false},
		{"unrelated knowledge word", status.Error(codes.Internal, "KNOWLEDGE_SKILL_NAMES differ"), nil, []error{ErrKnowledgeHubUnavailable, ErrReadonlyUnsupported, ErrTimeout}, false},
		{"invalid option name is not a root problem", status.Error(codes.InvalidArgument, "INVALID_KNOWLEDGE_ROOT"), nil, []error{ErrKnowledgeHubUnavailable}, false},
		{"plain failure", plain, plain, []error{ErrTimeout, ErrReadonlyUnsupported, ErrKnowledgeHubUnavailable}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyExecError(tc.err)
			if tc.is != nil && !errors.Is(got, tc.is) {
				t.Errorf("err = %v, want Is %v", got, tc.is)
			}
			for _, not := range tc.isNot {
				if errors.Is(got, not) {
					t.Errorf("err = %v must not be %v", got, not)
				}
			}
			if tc.bareIs && got != tc.is {
				t.Errorf("err = %#v, want exactly %v", got, tc.is)
			}
		})
	}
	t.Run("generic wrap names the method and keeps the status", func(t *testing.T) {
		raw := status.Error(codes.Unavailable, "down")
		got := classifyExecError(raw)
		if status.Code(got) != codes.Unavailable || !errors.Is(got, raw) || got.Error() == raw.Error() {
			t.Errorf("err = %v", got)
		}
	})
	t.Run("knowledge refusal keeps the cause", func(t *testing.T) {
		raw := status.Error(codes.FailedPrecondition, "KNOWLEDGE_HUB_UNSUPPORTED")
		if got := classifyExecError(raw); !errors.Is(got, raw) {
			t.Errorf("cause lost: %v", got)
		}
	})
}

func TestClassifyCompleteError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		timeout  bool
	}{
		{"deadline", status.Error(codes.DeadlineExceeded, "x"), "", true},
		{"typed", status.Error(codes.Unavailable, "AI_COMPLETE_BUSY: try later"), "AI_COMPLETE_BUSY", false},
		{"typed without detail text", status.Error(codes.Unavailable, "AI_COMPLETE_SESSION_EXPIRED: "), "AI_COMPLETE_SESSION_EXPIRED", false},
		{"colon in free text", status.Error(codes.Internal, "something failed: details"), "", false},
		{"lowercase code is not a code", status.Error(codes.Internal, "AI_COMPLETE_busy: no"), "", false},
		{"not a status error", errors.New("AI_COMPLETE_BUSY: nope"), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyCompleteError(tc.err)
			var ce *CompleteError
			isTyped := errors.As(got, &ce)
			switch {
			case tc.timeout:
				if got != ErrTimeout {
					t.Errorf("err = %v, want ErrTimeout", got)
				}
			case tc.wantCode != "":
				if !isTyped || ce.Code != tc.wantCode {
					t.Errorf("err = %v, want code %s", got, tc.wantCode)
				}
			default:
				if isTyped || !errors.Is(got, tc.err) {
					t.Errorf("err = %v, want generic wrap", got)
				}
			}
		})
	}
}
