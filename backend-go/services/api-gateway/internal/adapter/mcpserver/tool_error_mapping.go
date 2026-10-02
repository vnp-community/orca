package mcpserver

import (
	"regexp"

	"google.golang.org/grpc/status"
)

var toolErrorPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+: .+`)

const genericToolFailure = "The operation failed."

// MapToolError turns an execution error into the short text of an
// isError:true tool result. Only a "CODE: message" gRPC status message (the
// shape apperrors.ToGRPCStatus emits) passes through; everything else —
// "rpc error:" wrappers, stacks, other services' names — collapses to a
// generic sentence so internals never leak to the LLM client.
func MapToolError(err error) string {
	if err == nil {
		return genericToolFailure
	}
	msg := status.Convert(err).Message()
	if toolErrorPattern.MatchString(msg) && len(msg) <= 300 {
		return msg
	}
	return genericToolFailure
}
