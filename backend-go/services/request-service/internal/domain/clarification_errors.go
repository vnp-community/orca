package domain

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrClarificationNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_CLARIFICATION_NOT_FOUND", fmt.Sprintf("clarification not found: %s", id), nil)
}

func ErrClarificationNotOpen(id string, status ClarificationStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_NOT_OPEN",
		fmt.Sprintf("clarification %s is %s, not open", id, status), nil)
}

func ErrClarificationExpired(id string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_EXPIRED", fmt.Sprintf("clarification %s is past its due time", id), nil)
}

func ErrClarificationAlreadyAnswered(id string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_ALREADY_ANSWERED",
		fmt.Sprintf("clarification %s was already answered with different content", id), nil)
}

func ErrClarificationIncomplete(missingKeys []string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_INCOMPLETE",
		"required questions are unanswered: "+strings.Join(missingKeys, ", "), nil)
}

func ErrClarificationNotAssignee() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_CLARIFICATION_NOT_ASSIGNEE", "only an assignee or an admin may answer this clarification", nil)
}

func ErrClarificationInvalidAnswer(questionKey, reason string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_CLARIFICATION_INVALID_ANSWER",
		fmt.Sprintf("question %q: %s", questionKey, reason), nil)
}

func ErrClarificationStateNotAllowed(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED", reason, nil)
}

func ErrClarificationVersionConflict(id string, expected int64) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLARIFICATION_VERSION_CONFLICT",
		fmt.Sprintf("clarification %s version conflict (expected %d)", id, expected), nil)
}

func ErrReadinessWaiveForbidden(reason string) error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_READINESS_WAIVE_FORBIDDEN", reason, nil)
}

func ErrDecisionNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_DECISION_NOT_FOUND", fmt.Sprintf("decision not found: %s", id), nil)
}

func ErrDecisionRationaleRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_DECISION_RATIONALE_REQUIRED",
		"a rationale is required when the chosen option is not the recommended one", nil)
}

func ErrDecisionNotEffective(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_DECISION_NOT_EFFECTIVE", reason, nil)
}

func ErrDecisionConfirmationMismatch() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_DECISION_CONFIRMATION_MISMATCH",
		"confirmation text does not match the chosen option title", nil)
}

func ErrDecisionAgentForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_DECISION_AGENT_FORBIDDEN", "a machine identity cannot confirm a decision", nil)
}

func ErrDecisionSelfChoiceForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_DECISION_SELF_CHOICE_FORBIDDEN", "the reporter may not choose for their own request", nil)
}

func ErrDecisionStateInvalid(id string, status DecisionStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_DECISION_STATE_INVALID",
		fmt.Sprintf("decision %s is %s", id, status), nil)
}

func ErrDecisionVersionConflict(id string, expected int64) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_DECISION_VERSION_CONFLICT",
		fmt.Sprintf("decision %s version conflict (expected %d)", id, expected), nil)
}

func ErrDecisionLiveExists(subject string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_DECISION_STATE_INVALID",
		fmt.Sprintf("a live decision already exists for %s", subject), nil)
}

func ErrSolutionBlockingQuestions(ids []string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_BLOCKING_QUESTIONS",
		"blocking open questions are unanswered: "+strings.Join(ids, ", "), nil)
}

func ErrPlanUnconfirmedAssumptions(ids []string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS",
		"assumptions needing confirmation are unanswered: "+strings.Join(ids, ", "), nil)
}
