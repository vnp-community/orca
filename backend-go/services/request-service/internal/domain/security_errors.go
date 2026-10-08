package domain

import (
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ErrRequestForbidden is the one answer for every refused access so a caller cannot tell why.
func ErrRequestForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_FORBIDDEN", "caller is not allowed to perform this action", nil)
}

func ErrRequestPolicyUnavailable(cause error) error {
	return apperrors.New(apperrors.KindInternal, "REQUEST_POLICY_EVAL_FAILED", "failed to evaluate the authorization policy", cause)
}

func ErrProjectRoleUnavailable(cause error) error {
	return apperrors.New(apperrors.KindUnavailable, "REQUEST_PROJECT_ROLE_UNAVAILABLE", "project membership cannot be resolved right now", cause)
}

// RateLimitedError carries the wait the caller should honor; detail is "rate" or "concurrency".
type RateLimitedError struct {
	Class      string
	Detail     string
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return "REQUEST_RATE_LIMITED: " + e.Class + "/" + e.Detail }

func IsRateLimited(err error) (*RateLimitedError, bool) {
	var rl *RateLimitedError
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}

func ErrRequestPayloadTooLarge(field string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PAYLOAD_TOO_LARGE", field+" exceeds the allowed size", nil)
}

func ErrEraseNotAllowed() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ERASE_NOT_ALLOWED", "a request that is executing cannot be erased", nil)
}

func ErrEraseReasonRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ERASE_REASON_REQUIRED", "reason is required (at most 500 characters)", nil)
}

func ErrEraseKeyMissing() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ERASE_KEY_MISSING", "erase key is not configured", nil)
}

func ErrExportTooLarge() error {
	return apperrors.New(apperrors.KindResourceExhausted, "REQUEST_EXPORT_TOO_LARGE", "export exceeds the size limit", nil)
}

// ErrErasureUnsupported marks an external system that has no erase RPC yet; EraseRequest reports it instead of failing.
var ErrErasureUnsupported = errors.New("erasure unsupported")
