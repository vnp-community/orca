// Package apperrors defines the domain-error taxonomy shared at the gRPC
// boundary and its mapping to gRPC status codes — one place, not scattered
// per service. See specs/backend-go/architecture/03-clean-architecture-guidelines.md
// ("adapter layer is the only place a domain error gets mapped to a wire
// status code") and standards/go-coding-standards.md's error-handling section.
package apperrors

import (
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Kind categorizes a domain error into the handful of shapes every service's
// domain/usecase layers actually need — deliberately small and closed, not
// an open string, so the gRPC mapping in ToGRPCStatus can't silently drop a
// new kind.
type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindAlreadyExists
	KindInvalidArgument
	KindPermissionDenied
	KindFailedPrecondition // e.g. cyclic task dependency, rebind while active execution
	KindUnauthenticated
	KindInternal
	// KindDeadlineExceeded is an outer deadline expiring around a
	// multi-step operation (e.g. automation-service's BR-AT-06 2-hour
	// RunNow budget) — distinct from KindInternal so callers can tell a
	// timeout apart from an ordinary failure.
	KindDeadlineExceeded
	KindUnavailable
	KindResourceExhausted
)

// AppError is the typed error every domain/ package returns instead of a
// bare fmt.Errorf — carries a stable machine-readable Code (for API
// consumers, per standards/api-design-guidelines.md's error model) plus a
// Kind (for the gRPC status mapping).
type AppError struct {
	Kind Kind
	// Code is a stable, machine-readable identifier (e.g. "TASK_CYCLIC_DEPENDENCY")
	// — clients must key behavior off this, never off Message.
	Code    string
	Message string
	Err     error // wrapped cause, if any
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// New constructs an AppError. Domain packages should define package-level
// sentinel-style constructors (e.g. ErrTaskNotFound) that call this, rather
// than every call site building an AppError inline.
func New(kind Kind, code, message string, cause error) *AppError {
	return &AppError{Kind: kind, Code: code, Message: message, Err: cause}
}

// logger is an optional package-level sink for the wrapped cause
// ToGRPCStatus otherwise discards — nil until a service's main.go calls
// SetLogger, in which case every call site is a no-op (same
// wire-once-from-main-nil-safe-until-then convention as
// project-service/internal/usecase/authorization.go's auditClient).
var logger *slog.Logger

// SetLogger wires an optional logger for ToGRPCStatus to record an
// AppError's wrapped cause to before it's discarded (see ToGRPCStatus's doc
// comment for why that cause never reaches the client). Call once, from
// main.go, before serving traffic. Never required — omitting this call
// keeps ToGRPCStatus's return value identical to before this existed.
func SetLogger(l *slog.Logger) { logger = l }

// ToGRPCStatus maps an AppError to a gRPC status — the single mapping table
// standards/api-design-guidelines.md requires, imported by every service's
// adapter/grpc/ layer instead of each reimplementing this switch.
//
// BUG-009 (specs/backend-go/bugs/missing-v2/): ae.Err (the real underlying
// cause — e.g. a devserveragent transport failure) is deliberately never
// included in the returned status (ae.Code+": "+ae.Message only) — clients
// must key behavior off the stable Code, never off wrapped internal detail,
// per this function's own doc comment above. But that same omission also
// meant the cause never reached even this service's OWN structured logs
// (common/grpcmw.LoggingInterceptor logs the error THIS function returns,
// after the cause is already gone) — undiagnosable from live logs alone, as
// confirmed investigating BUG-009 live against b15.openledger.vn. Logging it
// here, gated behind the nil-by-default logger above, fixes that without
// changing what any of this function's 463 existing call sites return.
func ToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	var ae *AppError
	if !errors.As(err, &ae) {
		return status.Error(codes.Internal, "internal error")
	}
	if ae.Err != nil && logger != nil {
		logger.Error("apperrors: internal cause (not sent to client)",
			"code", ae.Code,
			"cause", ae.Err.Error(),
		)
	}
	code := codes.Unknown
	switch ae.Kind {
	case KindNotFound:
		code = codes.NotFound
	case KindAlreadyExists:
		code = codes.AlreadyExists
	case KindInvalidArgument:
		code = codes.InvalidArgument
	case KindPermissionDenied:
		code = codes.PermissionDenied
	case KindFailedPrecondition:
		code = codes.FailedPrecondition
	case KindUnauthenticated:
		code = codes.Unauthenticated
	case KindDeadlineExceeded:
		code = codes.DeadlineExceeded
	case KindUnavailable:
		code = codes.Unavailable
	case KindResourceExhausted:
		code = codes.ResourceExhausted
	case KindInternal, KindUnknown:
		code = codes.Internal
	}
	return status.Error(code, ae.Code+": "+ae.Message)
}
