package wscompat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCodeIntelChannelError(t *testing.T) {
	// Status Code Mappings
	tests := []struct {
		err  error
		want string
	}{
		{status.Error(codes.Unavailable, "boom /home/dev/proj/secret.go"), "CODEINTEL_UNAVAILABLE: code-intel unavailable"},
		{status.Error(codes.Unimplemented, "boom"), "CODEINTEL_UNAVAILABLE: code-intel unavailable"},
		{status.Error(codes.DeadlineExceeded, "boom"), `CODEINTEL_TIMEOUT: code-intel did not respond in time | {"retryAfterMs":3000,"inProgress":true}`},
		{status.Error(codes.Canceled, "boom"), "CODEINTEL_TIMEOUT: request cancelled"},
		{status.Error(codes.NotFound, "boom"), "CODEINTEL_NOT_AUTHORIZED: not found or not permitted"},
		{status.Error(codes.PermissionDenied, "boom"), "CODEINTEL_NOT_AUTHORIZED: not found or not permitted"},
		{status.Error(codes.InvalidArgument, "boom"), "CODEINTEL_INVALID_PARAMS: invalid params"},
		{status.Error(codes.FailedPrecondition, "boom"), "CODEINTEL_TOOL_FAILED: tool failed"},
		{status.Error(codes.ResourceExhausted, "size exceeded"), "CODEINTEL_RESPONSE_TOO_LARGE: response too large"},
		{status.Error(codes.ResourceExhausted, "too many requests"), "CODEINTEL_RATE_LIMITED: rate limited"},
		{status.Error(codes.Aborted, "boom"), "CODEINTEL_VERSION_CONFLICT: version conflict"},
		{status.Error(codes.AlreadyExists, "boom"), "CODEINTEL_VERSION_CONFLICT: version conflict"},
		{status.Error(codes.Internal, "boom"), "CODEINTEL_INTERNAL: internal error"},
		{status.Error(codes.FailedPrecondition, "CODEINTEL_DISABLED: code intelligence is disabled"), "CODEINTEL_DISABLED: code intelligence is disabled"},
		{fmt.Errorf("wrap: %w", context.DeadlineExceeded), `CODEINTEL_TIMEOUT: code-intel did not respond in time | {"retryAfterMs":3000,"inProgress":true}`},
	}

	for _, tc := range tests {
		got := codeIntelChannelError(tc.err)
		if got == nil || got.Error() != tc.want {
			t.Errorf("codeIntelChannelError(%v) = %v; want %v", tc.err, got, tc.want)
		}
	}

	// Long strings, multi-lines, invalid JSON
	longHuman := strings.Repeat("A", 300)
	errLong := errors.New("CODEINTEL_AMBIGUOUS_SYMBOL: " + longHuman)
	gotLong := codeIntelChannelError(errLong).Error()
	if !strings.HasPrefix(gotLong, "CODEINTEL_AMBIGUOUS_SYMBOL: "+strings.Repeat("A", 200)) || len(gotLong) > 230 {
		t.Errorf("expected truncation, got %s", gotLong)
	}

	errNewline := errors.New("CODEINTEL_AMBIGUOUS_SYMBOL: a\nb\tc")
	if got := codeIntelChannelError(errNewline).Error(); got != "CODEINTEL_AMBIGUOUS_SYMBOL: a b c" {
		t.Errorf("expected whitespace normalization, got %s", got)
	}

	// Candidates truncation
	candidatesJSON := `{"candidates":[1,2,3,4,5,6,7,8,9,10,11,12]}`
	errCandidates := errors.New("CODEINTEL_AMBIGUOUS_SYMBOL: amb | " + candidatesJSON)
	gotCandidates := codeIntelChannelError(errCandidates).Error()
	if !strings.Contains(gotCandidates, `[1,2,3,4,5,6,7,8,9,10]`) {
		t.Errorf("expected candidates to be truncated to 10, got %s", gotCandidates)
	}

	// Invalid JSON suffix
	errInvalidJSON := errors.New("CODEINTEL_AMBIGUOUS_SYMBOL: amb | nope")
	if got := codeIntelChannelError(errInvalidJSON).Error(); got != "CODEINTEL_AMBIGUOUS_SYMBOL: amb" {
		t.Errorf("expected invalid JSON suffix to be dropped, got %s", got)
	}

	// Paths
	errPath := errors.New("CODEINTEL_TOOL_FAILED: fail /home/dev/x/y.go and C:\\Users\\a\\b\\c.go and https://host/a/b/c and /a/b")
	gotPath := codeIntelChannelError(errPath).Error()
	if gotPath != "CODEINTEL_TOOL_FAILED: fail <path> and <path> and https://host/a/b/c and /a/b" {
		t.Errorf("expected paths to be scrubbed, got %s", gotPath)
	}

	// gRPC Error Wrapper
	errWrap := errors.New(`rpc error: code = Unavailable desc = CODEINTEL_TOOL_UNAVAILABLE: x | {"tool":"gitnexus","reason":"unsupported_version"}`)
	gotWrap := codeIntelChannelError(errWrap).Error()
	if gotWrap != `CODEINTEL_TOOL_UNAVAILABLE: x | {"reason":"unsupported_version","tool":"gitnexus"}` {
		t.Errorf("expected grpc wrapper removal, got %s", gotWrap)
	}
}
