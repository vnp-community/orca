package domain

// DeriveContainerStatus computes a plan/phase status from its children's
// statuses. ok=false means "no children: keep the current status".
// Cancelled children are ignored so a dropped task never blocks completion.
func DeriveContainerStatus(children []Status) (Status, bool) {
	if len(children) == 0 {
		return "", false
	}
	var open, blocked, inProgress, review, done int
	for _, s := range children {
		switch s {
		case StatusOpen:
			open++
		case StatusBlocked:
			blocked++
		case StatusInProgress:
			inProgress++
		case StatusReview:
			review++
		case StatusDone:
			done++
		}
	}
	live := open + blocked + inProgress + review + done
	switch {
	case live == 0:
		return StatusCancelled, true
	case done == live:
		return StatusDone, true
	case inProgress > 0:
		return StatusInProgress, true
	case open == 0 && blocked == 0:
		// only done/review remain and at least one review
		return StatusReview, true
	case done+review > 0:
		return StatusInProgress, true
	case blocked == live:
		return StatusBlocked, true
	default:
		return StatusOpen, true
	}
}
