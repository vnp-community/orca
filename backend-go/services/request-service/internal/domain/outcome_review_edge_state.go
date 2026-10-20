package domain

// EdgeState is how an edge, message or node compares with the base, the head and the design. An agent reports what it
// saw in each place; Orca derives the state, so no result can claim a state it did not support.
type EdgeState string

const (
	StateAdded     EdgeState = "added"
	StateRemoved   EdgeState = "removed"
	StateSame      EdgeState = "same"
	StateDeviation EdgeState = "deviation" // in the code but the design says it is not there
	StateMissing   EdgeState = "missing"   // the design has it but the code does not
	StateUnmapped  EdgeState = "unmapped"  // a node that is in neither the knowledge graph nor the SAD; source=code only
)

// Presence is what was observed about one edge, message or node.
type Presence struct {
	InBase bool
	InHead bool
	// InDesign is nil when there is no design to compare with.
	InDesign *bool
}

// DeriveState implements the three-way comparison. ok is false for a thing seen nowhere (not in base, head or design), which has
// no state and must be dropped.
func DeriveState(p Presence) (state EdgeState, change EdgeState, ok bool) {
	switch {
	case !p.InBase && p.InHead:
		change = StateAdded
	case p.InBase && !p.InHead:
		change = StateRemoved
	case p.InBase && p.InHead:
		change = StateSame
	}
	if p.InDesign != nil {
		if p.InHead && !*p.InDesign {
			return StateDeviation, change, true
		}
		if *p.InDesign && !p.InHead {
			return StateMissing, change, true
		}
	}
	if change == "" {
		return "", "", false
	}
	return change, change, true
}

// IsReviewable says whether a state is one a person has to look at.
func (s EdgeState) IsReviewable() bool {
	switch s {
	case StateAdded, StateRemoved, StateDeviation, StateMissing, StateUnmapped:
		return true
	}
	return false
}

// IsFinding is the subset of states that needs evidence to be believed: they all claim something about the code or the design.
func (s EdgeState) IsFinding() bool {
	switch s {
	case StateAdded, StateRemoved, StateDeviation, StateMissing:
		return true
	}
	return false
}

// AdvisoryFor says whether a finding rests on knowledge graph elements nobody approved. Every basis must be approved for a
// deviation or a missing edge to be able to ask for a decision.
func AdvisoryFor(state EdgeState, bases ...KGReviewState) bool {
	if state != StateDeviation && state != StateMissing {
		return false
	}
	if len(bases) == 0 {
		return true
	}
	for _, b := range bases {
		if b != KGApproved {
			return true
		}
	}
	return false
}
