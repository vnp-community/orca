package domain

// Visibility is a WorkflowTemplate's sharing tier — BUG-WF-03's
// escalate-forward publish state machine, see CanEscalateTo's doc comment.
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityTeam    Visibility = "team"
	VisibilityCompany Visibility = "company"
	VisibilityPublic  Visibility = "public"
)

// visibilityRank orders Visibility for escalation-only transitions — the
// state machine is escalate-forward (private -> team -> company -> public)
// with company/public requiring approval (see usecase.PublishTemplate);
// de-escalation (any tier -> private) is a separate, always-allowed
// "unpublish" operation, not subject to the one-tier-at-a-time rule.
var visibilityRank = map[Visibility]int{VisibilityPrivate: 0, VisibilityTeam: 1, VisibilityCompany: 2, VisibilityPublic: 3}

// Valid reports whether v is one of the four known visibility tiers.
func (v Visibility) Valid() bool {
	_, ok := visibilityRank[v]
	return ok
}

// CanEscalateTo reports whether moving from v to next is a valid single
// forward step (only one tier at a time) OR any direct de-escalation back
// to private (unpublish — always allowed, any distance, in one step).
func (v Visibility) CanEscalateTo(next Visibility) bool {
	if next == VisibilityPrivate {
		return true
	}
	return visibilityRank[next] == visibilityRank[v]+1
}
