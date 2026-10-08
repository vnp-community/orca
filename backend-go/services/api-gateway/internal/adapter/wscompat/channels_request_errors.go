package wscompat

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errRequestUnavailable is what every Request channel returns when
// REQUEST_SERVICE_ADDR is not configured (CONTRACT section 5).
var errRequestUnavailable = errors.New("REQUEST_UNAVAILABLE: request service not configured")

// requestClientMissing reports a nil interface or an interface holding a nil pointer.
func requestClientMissing(c any) bool {
	if c == nil {
		return true
	}
	v := reflect.ValueOf(c)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

var (
	requestCodedMessage = regexp.MustCompile(`^REQUEST_[A-Z0-9_]+: `)
	grpcErrorEnvelope   = regexp.MustCompile(`rpc error: code = [A-Za-z]+ desc = `)
)

// maxRequestErrorMessage bounds relayed service messages.
const maxRequestErrorMessage = 300

// requestChannelError turns a gRPC failure of the Request domain into
// "<CODE>: <message>" (CONTRACT C4). A service-issued REQUEST_* code is relayed
// as is; every other failure gets a fixed message so no input value (body,
// title) can reach the client through an error (CONTRACT C9).
func requestChannelError(err error) error { return requestErrorWith(err, "REQUEST_FORBIDDEN") }

// approvalChannelError is requestChannelError for approval.* channels, where a
// bare PermissionDenied means "not an approver".
func approvalChannelError(err error) error {
	return requestErrorWith(err, "REQUEST_APPROVAL_FORBIDDEN")
}

// RequestChannelError is the exported form used by the HTTP routes.
func RequestChannelError(err error) error { return requestChannelError(err) }

func requestErrorWith(err error, forbiddenCode string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if st, ok := status.FromError(err); ok {
		msg = st.Message()
	}
	if locs := grpcErrorEnvelope.FindAllStringIndex(msg, -1); len(locs) > 0 {
		msg = msg[locs[len(locs)-1][1]:]
	}
	msg = strings.TrimSpace(msg)
	if requestCodedMessage.MatchString(msg) {
		if len(msg) > maxRequestErrorMessage {
			msg = msg[:maxRequestErrorMessage]
		}
		return errors.New(msg)
	}
	code := status.Code(err)
	switch {
	case errors.Is(err, context.DeadlineExceeded), code == codes.DeadlineExceeded:
		return errors.New("REQUEST_UNAVAILABLE: request service did not respond in time")
	case code == codes.Unavailable:
		return errors.New("REQUEST_UNAVAILABLE: request service unavailable")
	case code == codes.Unimplemented:
		return errors.New("REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet")
	case code == codes.NotFound:
		return errors.New("REQUEST_NOT_FOUND: not found")
	case code == codes.PermissionDenied, code == codes.Unauthenticated:
		return errors.New(forbiddenCode + ": not permitted")
	case code == codes.ResourceExhausted:
		return errors.New("REQUEST_RATE_LIMITED: too many requests")
	case code == codes.InvalidArgument:
		return errors.New("INVALID_ARGUMENT: invalid argument")
	case code == codes.FailedPrecondition, code == codes.Aborted, code == codes.AlreadyExists:
		return errors.New("REQUEST_STATE_STALE: the operation is not valid in the current state")
	default:
		return errors.New("REQUEST_INTERNAL: internal error")
	}
}
