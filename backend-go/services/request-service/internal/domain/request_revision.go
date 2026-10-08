package domain

import (
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// RevisionCause says why a Request content revision exists.
type RevisionCause string

const (
	RevisionCauseCreated               RevisionCause = "created"
	RevisionCauseClarificationAnswered RevisionCause = "clarification_answered"
	RevisionCauseEdited                RevisionCause = "edited"
	RevisionCauseTypeChanged           RevisionCause = "type_changed"
)

func AllRevisionCauses() []RevisionCause {
	return []RevisionCause{RevisionCauseCreated, RevisionCauseClarificationAnswered, RevisionCauseEdited, RevisionCauseTypeChanged}
}

func ParseRevisionCause(s string) (RevisionCause, error) {
	for _, c := range AllRevisionCauses() {
		if string(c) == s {
			return c, nil
		}
	}
	return "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_REVISION_CAUSE_INVALID", fmt.Sprintf("invalid revision cause: %s", s), nil)
}

// RevisionActorKind is the actor vocabulary of request_revisions (ai, not the lifecycle's "agent").
type RevisionActorKind string

const (
	RevisionActorAI     RevisionActorKind = "ai"
	RevisionActorUser   RevisionActorKind = "user"
	RevisionActorSystem RevisionActorKind = "system"
)

func ParseRevisionActorKind(s string) (RevisionActorKind, error) {
	switch RevisionActorKind(s) {
	case RevisionActorAI, RevisionActorUser, RevisionActorSystem:
		return RevisionActorKind(s), nil
	}
	return "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_REVISION_ACTOR_INVALID", fmt.Sprintf("invalid revision actor kind: %s", s), nil)
}

// RevisionActorFrom maps the lifecycle actor kind onto the revision vocabulary.
func RevisionActorFrom(k ActorKind) RevisionActorKind {
	switch k {
	case ActorKindAgent:
		return RevisionActorAI
	case ActorKindSystem:
		return RevisionActorSystem
	}
	return RevisionActorUser
}

// RequestRevision is an immutable, append-only snapshot of Request content.
type RequestRevision struct {
	ID              string
	TenantID        string
	RequestID       string
	Revision        int
	Cause           RevisionCause
	Snapshot        []byte
	Digest          string
	ActorID         string
	ActorKind       RevisionActorKind
	ClarificationID string
	CreatedAt       time.Time
}

func ErrRequestRevisionNotFound(requestID string, revision int) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_REVISION_NOT_FOUND", fmt.Sprintf("revision %d of request %s not found", revision, requestID), nil)
}
