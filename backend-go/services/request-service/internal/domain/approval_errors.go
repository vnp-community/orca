package domain

import "github.com/stablyai/orca-go/common/apperrors"

// Sentinels are *AppError values so callers can match with errors.Is and the gRPC layer maps them without a switch.
var (
	ErrApprovalNotFound              = apperrors.New(apperrors.KindNotFound, "REQUEST_APPROVAL_NOT_FOUND", "approval not found", nil)
	ErrApprovalNotPending            = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_ALREADY_DECIDED", "approval is no longer pending", nil)
	ErrApprovalExpired               = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_EXPIRED", "approval expired", nil)
	ErrApprovalVersionConflict       = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_VERSION_CONFLICT", "approval version conflict", nil)
	ErrApprovalDigestMismatch        = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_DIGEST_MISMATCH", "subject changed since the approval was shown", nil)
	ErrApprovalStageMismatch         = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_STAGE_MISMATCH", "request is no longer at the approval stage", nil)
	ErrApprovalPendingExists         = apperrors.New(apperrors.KindAlreadyExists, "REQUEST_APPROVAL_PENDING_EXISTS", "a pending approval already exists for this subject", nil)
	ErrApprovalIdempotency           = apperrors.New(apperrors.KindAlreadyExists, "REQUEST_APPROVAL_IDEMPOTENCY_CONFLICT", "idempotency key already used for another approval", nil)
	ErrApprovalSubjectNotFound       = apperrors.New(apperrors.KindNotFound, "REQUEST_APPROVAL_SUBJECT_NOT_FOUND", "approval subject not found", nil)
	ErrApprovalSubjectTypeNotAllowed = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED", "subject type not allowed for this request", nil)
	ErrApprovalSubjectUnavailable    = apperrors.New(apperrors.KindUnavailable, "REQUEST_APPROVAL_SUBJECT_UNAVAILABLE", "subject handler has no backing service wired", nil)
	ErrApprovalCommentRequired       = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_COMMENT_REQUIRED", "comment is required", nil)
	ErrApprovalCommentTooLong        = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_COMMENT_TOO_LONG", "comment is too long", nil)
	ErrApprovalSubjectTypeInvalid    = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_SUBJECT_TYPE_INVALID", "unknown subject type", nil)
	ErrApprovalDecisionInvalid       = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_DECISION_INVALID", "decision must be approve or reject", nil)
	ErrApprovalExtendInvalid         = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_EXTEND_INVALID", "extension must be between 1 second and 30 days", nil)
	ErrApprovalForbidden             = apperrors.New(apperrors.KindPermissionDenied, "REQUEST_APPROVAL_FORBIDDEN", "not allowed to perform this action on the approval", nil)
	ErrApprovalNoEligibleApprover    = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER", "no eligible approver for this approval", nil)
	ErrApprovalPolicyInvalid         = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_POLICY_INVALID", "approval policy is invalid", nil)
	ErrApprovalPolicyNotFound        = apperrors.New(apperrors.KindNotFound, "REQUEST_APPROVAL_POLICY_NOT_FOUND", "approval policy not found", nil)
	ErrApprovalDirectoryUnavailable  = apperrors.New(apperrors.KindUnavailable, "REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE", "team or user directory is unavailable", nil)
)
