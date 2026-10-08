package domain

import "fmt"

const (
	MaxChildrenPerParent = 50
	// MaxAncestorDepth is the deepest nesting level a child may be created at (the root is level 1).
	MaxAncestorDepth = 5
)

// ValidateChild applies the parent/child table of CR-REQ-006 section 2.6.
// An empty typeHint is always accepted: the child then goes through normal classification.
func ValidateChild(parent Request, reason LinkReason, typeHint RequestType) error {
	if parent.Status == RequestStatusCancelled {
		return ErrChildNotAllowed("a cancelled request cannot spawn children")
	}
	var (
		parentType RequestType
		statuses   []RequestStatus
		hints      []RequestType
	)
	switch reason {
	case LinkReasonSpawnedBySpike:
		parentType, statuses, hints = RequestTypeSpike, []RequestStatus{RequestStatusAwaitingAnalysisApproval, RequestStatusCompleted}, []RequestType{RequestTypeChangeRequest, RequestTypeTask}
	case LinkReasonSpawnedByQuestion:
		parentType, statuses, hints = RequestTypeQuestion, []RequestStatus{RequestStatusAwaitingAnalysisApproval, RequestStatusCompleted}, []RequestType{RequestTypeChangeRequest, RequestTypeTask}
	case LinkReasonFollowupHotfix:
		parentType, statuses, hints = RequestTypeHotfix, []RequestStatus{RequestStatusExecuting, RequestStatusCompleted}, []RequestType{RequestTypeBug, RequestTypeTask}
	case LinkReasonEscalation:
		if typeHint != "" {
			if _, err := ParseRequestType(string(typeHint)); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrChildNotAllowed(fmt.Sprintf("link reason %q cannot create a child request", reason))
	}
	if parent.Type != parentType {
		return ErrChildNotAllowed(fmt.Sprintf("%s needs a %s parent, got %q", reason, parentType, parent.Type))
	}
	if !statusIn(parent.Status, statuses...) {
		return ErrChildNotAllowed(fmt.Sprintf("%s needs the parent in %v, got %q", reason, statuses, parent.Status))
	}
	if typeHint != "" {
		ok := false
		for _, h := range hints {
			ok = ok || h == typeHint
		}
		if !ok {
			return ErrChildNotAllowed(fmt.Sprintf("%s allows type hints %v, got %q", reason, hints, typeHint))
		}
	}
	return nil
}
