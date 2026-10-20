package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Codes of the Outcome Review feature. A knowledge graph that is absent is never an error: it is the warning kg_unavailable.
const (
	CodeReviewDisabled          = "REQUEST_REVIEW_DISABLED"
	CodeReviewNotFound          = "REQUEST_REVIEW_NOT_FOUND"
	CodeReviewNotReady          = "REQUEST_REVIEW_NOT_READY"
	CodeReviewRunInProgress     = "REQUEST_REVIEW_RUN_IN_PROGRESS"
	CodeReviewSkillMissing      = "REQUEST_REVIEW_SKILL_MISSING"
	CodeReviewResultInvalid     = "REQUEST_REVIEW_RESULT_INVALID"
	CodeReviewArtifactUnsafe    = "REQUEST_REVIEW_ARTIFACT_UNSAFE"
	CodeReviewArtifactTooLarge  = "REQUEST_REVIEW_ARTIFACT_TOO_LARGE"
	CodeReviewProfileInvalid    = "REQUEST_REVIEW_PROFILE_INVALID"
	CodeReviewBudgetExceeded    = "REQUEST_REVIEW_BUDGET_EXCEEDED"
	CodeReviewStaleRevision     = "REQUEST_REVIEW_STALE_REVISION"
	CodeReviewDecisionsOpen     = "REQUEST_REVIEW_DECISIONS_OPEN"
	CodeReviewDecisionInvalid   = "REQUEST_REVIEW_DECISION_INVALID"
	CodeReviewArtifactNotFound  = "REQUEST_REVIEW_ARTIFACT_NOT_FOUND"
	CodeReviewArtifactCorrupt   = "REQUEST_REVIEW_ARTIFACT_CORRUPT"
	CodeReviewKnowledgeRequired = "REQUEST_REVIEW_KNOWLEDGE_REQUIRED"
	CodeReviewAgentTooOld       = "REQUEST_REVIEW_AGENT_TOO_OLD"
)

func ErrReviewDisabled() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewDisabled, "outcome review is not enabled for this project", nil)
}

func ErrReviewNotFound(id string) error {
	return newNotFound(CodeReviewNotFound, "outcome review not found: "+id)
}

func ErrReviewNotReady(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewNotReady,
		fmt.Sprintf("a request that is %q has no plan to review yet", status), nil)
}

func ErrReviewPhaseNotReady(phase ReviewPhase, what string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewNotReady,
		fmt.Sprintf("%s is not available in the %s phase of a review", what, phase), nil)
}

func ErrReviewRunInProgress(runID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewRunInProgress, "a review run is already in progress: "+runID, nil)
}

func ErrReviewSkillMissing(skill string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewSkillMissing,
		fmt.Sprintf("the dev server does not have the %s skill installed at the pinned version", skill), nil)
}

func ErrReviewResultInvalid(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewResultInvalid, "the agent result is not usable: "+reason, nil)
}

func ErrReviewArtifactUnsafe(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewArtifactUnsafe, "the drawing was refused: "+reason, nil)
}

func ErrReviewArtifactTooLarge(size, max int) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewArtifactTooLarge,
		fmt.Sprintf("the drawing is %d bytes; the limit is %d", size, max), nil)
}

func ErrReviewArtifactNotFound(ref string) error {
	return newNotFound(CodeReviewArtifactNotFound, "review artifact not found: "+ref)
}

func ErrReviewArtifactCorrupt(ref string) error {
	return apperrors.New(apperrors.KindInternal, CodeReviewArtifactCorrupt, "the stored drawing does not match its digest: "+ref, nil)
}

func ErrReviewProfileInvalid(path, reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeReviewProfileInvalid, fmt.Sprintf("drawing profile %s: %s", path, reason), nil)
}

func ErrReviewBudgetExceeded(reason string) error {
	return apperrors.New(apperrors.KindResourceExhausted, CodeReviewBudgetExceeded, reason, nil)
}

func ErrReviewStaleRevision() error {
	return apperrors.New(apperrors.KindAborted, CodeReviewStaleRevision, "the review changed since you loaded it; reload and try again", nil)
}

func ErrReviewDecisionsOpen(n int) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewDecisionsOpen,
		fmt.Sprintf("%d decision(s) still need an answer before the review can be approved", n), nil)
}

func ErrReviewDecisionInvalid(reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeReviewDecisionInvalid, reason, nil)
}

func ErrReviewAlreadyExists(requestID string) error {
	return apperrors.New(apperrors.KindAlreadyExists, "REQUEST_REVIEW_ALREADY_EXISTS", "a review for this request and code already exists: "+requestID, nil)
}

func ErrReviewNoDevServer(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_REVIEW_NO_DEV_SERVER", "no dev server can run the review: "+reason, nil)
}

func ErrReviewAgentTooOld(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewAgentTooOld, "the dev server agent cannot run the review: "+reason, nil)
}

func ErrReviewKnowledgeRequired() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeReviewKnowledgeRequired,
		"the design review needs the project's knowledge graph, which is not available", nil)
}
