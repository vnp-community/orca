package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// BaseRefLockReason says why the base ref of a Request can no longer change.
type BaseRefLockReason string

const (
	BaseRefUnlocked       BaseRefLockReason = ""
	BaseRefLockedProposal BaseRefLockReason = "proposal" // an OpenSpec change worktree exists
	BaseRefLockedTask     BaseRefLockReason = "task"     // a task of the Request has a worktree
	// BaseRefLockedExecution: task worktrees are created in task-service the moment a task is dispatched, which only happens
	// while the Request is executing; locking for the whole stage closes the gap before task-service has recorded one.
	BaseRefLockedExecution BaseRefLockReason = "execution"
	BaseRefLockedTerminal  BaseRefLockReason = "terminal" // the Request is completed or cancelled
)

// BaseRefError is an AppError that also carries machine-readable details (suggestions, lock reason) for the gRPC layer.
type BaseRefError struct {
	*apperrors.AppError
	Details map[string]string
}

func (e *BaseRefError) Unwrap() error { return e.AppError }

// BaseRefDetails returns the details of the BaseRefError in err's chain, if any.
func BaseRefDetails(err error) (map[string]string, bool) {
	var be *BaseRefError
	if errors.As(err, &be) {
		return be.Details, true
	}
	return nil, false
}

// ErrRequestBaseRefInvalid reports a ref that breaks the syntax rules; the value itself is not echoed.
func ErrRequestBaseRefInvalid(reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BASE_REF_INVALID", "base ref "+reason, nil)
}

// MaxBaseRefSuggestions bounds the "did you mean" list sent with REQUEST_BASE_REF_NOT_FOUND.
const MaxBaseRefSuggestions = 5

// ErrRequestBaseRefNotFound reports a ref git says does not exist; suggestions are the closest known refs.
func ErrRequestBaseRefNotFound(ref string, suggestions []string) error {
	if len(suggestions) > MaxBaseRefSuggestions {
		suggestions = suggestions[:MaxBaseRefSuggestions]
	}
	msg := fmt.Sprintf("base ref %q does not exist in the repository", ref)
	details := map[string]string{"ref": ref}
	if len(suggestions) > 0 {
		details["suggestions"] = strings.Join(suggestions, ",")
	}
	return &BaseRefError{AppError: apperrors.New(apperrors.KindNotFound, "REQUEST_BASE_REF_NOT_FOUND", msg, nil), Details: details}
}

// ErrRequestBaseRefLocked reports a change attempted after a worktree exists (or the Request is finished).
func ErrRequestBaseRefLocked(reason BaseRefLockReason) error {
	return &BaseRefError{
		AppError: apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_BASE_REF_LOCKED",
			"base ref can no longer change (reason: "+string(reason)+")", nil),
		Details: map[string]string{"reason": string(reason)},
	}
}

// ErrRequestBaseRefCheckNotConfigured is a deployment defect: a ref was submitted but this server has no way to check refs.
// Failing is deliberate; accepting the ref unchecked would turn a missing wiring into silent behaviour.
func ErrRequestBaseRefCheckNotConfigured() error {
	return apperrors.New(apperrors.KindInternal, "REQUEST_BASE_REF_CHECK_NOT_CONFIGURED", "this server cannot check base refs", nil)
}

// StoredBaseRefLock is the lock reason request-service can decide from its own tables: the Request status and whether a
// proposal worktree is recorded. A finished Request outranks everything; both dialects call this so they cannot differ.
func StoredBaseRefLock(status RequestStatus, proposalWorktree bool) BaseRefLockReason {
	switch {
	case status.IsTerminal():
		return BaseRefLockedTerminal
	case proposalWorktree:
		return BaseRefLockedProposal
	case status == RequestStatusExecuting:
		return BaseRefLockedExecution
	}
	return BaseRefUnlocked
}

// IsBaseRefNotFound reports whether err says git does not know the base ref: nothing was created from it.
func IsBaseRefNotFound(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == "REQUEST_BASE_REF_NOT_FOUND"
}
