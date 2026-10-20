package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// What a person can do about one item of a review that is after code.
type ReviewDecision string

const (
	DecisionAcceptException ReviewDecision = "accept_exception"
	DecisionSendBack        ReviewDecision = "send_back"
	DecisionDismiss         ReviewDecision = "dismiss"

	MaxDecisionNoteRunes = 500
)

func (d ReviewDecision) Valid() bool {
	return d == DecisionAcceptException || d == DecisionSendBack || d == DecisionDismiss
}

// NoteRequired says whether the person has to write why: accepting an exception and dismissing a claim are choices someone later
// will ask about; sending back is a request, and the item says what it is about.
func (d ReviewDecision) NoteRequired() bool {
	return d == DecisionAcceptException || d == DecisionDismiss
}

// ItemDecision is a decision in force or, when Active is false, one that was replaced by a later one.
type ItemDecision struct {
	ID             string
	ReviewID       string
	RequestID      string
	ItemKey        string
	Decision       ReviewDecision
	Note           string
	ChildRequestID string
	DecidedBy      string
	DecidedAt      time.Time
	Active         bool
	// Orphaned is set on read: the item the decision was about is no longer in the review, after a regeneration. The decision is kept.
	Orphaned bool
}

// CheckDecision applies the rules to one decision on one item of the review as it stands. It returns the item.
//
//   - only an after-code review has decisions (a design review has no code to decide about);
//   - a review that is stale or replaced cannot be decided on: regenerate first;
//   - accept_exception is only for an item that asks for a decision, and dismiss only for something an agent claimed, never a fact Orca measured;
//   - accept_exception and dismiss need a note of at most MaxDecisionNoteRunes.
func CheckDecision(r OutcomeReview, itemKey string, d ReviewDecision, note string) (ReviewItem, error) {
	if r.Phase != ReviewPhaseDelivered {
		return ReviewItem{}, ErrReviewPhaseNotReady(r.Phase, "a decision")
	}
	if r.State == ReviewStateStale || r.State == ReviewStateSuperseded {
		return ReviewItem{}, ErrReviewStaleRevision()
	}
	if !d.Valid() {
		return ReviewItem{}, ErrReviewDecisionInvalid("decision must be accept_exception, send_back or dismiss")
	}
	var item ReviewItem
	found := false
	for _, it := range r.Items {
		if it.Key == itemKey {
			item, found = it, true
			break
		}
	}
	if !found {
		return ReviewItem{}, ErrReviewDecisionInvalid("the review has no item " + itemKey)
	}
	note = strings.TrimSpace(note)
	switch {
	case utf8.RuneCountInString(note) > MaxDecisionNoteRunes:
		return ReviewItem{}, ErrReviewDecisionInvalid("the note is longer than 500 characters")
	case d.NoteRequired() && note == "":
		return ReviewItem{}, ErrReviewDecisionInvalid("a note is required to " + strings.ReplaceAll(string(d), "_", " "))
	case d == DecisionAcceptException && !item.NeedsDecision:
		return ReviewItem{}, ErrReviewDecisionInvalid("only an item that needs a decision can be accepted as an exception")
	case d == DecisionDismiss && item.Origin != OriginAgent:
		return ReviewItem{}, ErrReviewDecisionInvalid("only something an agent claimed can be dismissed; a measured fact cannot")
	}
	return item, nil
}

// OpenDecisions are the items that ask for a decision and have none in force. send_back counts as a decision: the item is handed to
// someone else, and a review with such items can be approved.
func OpenDecisions(r OutcomeReview, inForce []ItemDecision) []ReviewItem {
	decided := map[string]bool{}
	for _, d := range inForce {
		if d.Active {
			decided[d.ItemKey] = true
		}
	}
	var open []ReviewItem
	for _, it := range r.Items {
		if it.NeedsDecision && !decided[it.Key] {
			open = append(open, it)
		}
	}
	return open
}

// MarkOrphans flags the decisions whose item is gone from the review, so a screen can show them apart instead of losing them.
func MarkOrphans(r OutcomeReview, ds []ItemDecision) []ItemDecision {
	have := map[string]bool{}
	for _, it := range r.Items {
		have[it.Key] = true
	}
	out := make([]ItemDecision, len(ds))
	for i, d := range ds {
		d.Orphaned = !have[d.ItemKey]
		out[i] = d
	}
	return out
}

// NewDecisionsRequired says whether the review has an item that asks for a decision that nobody made: what takes an approved review
// back to ready after a regeneration.
func NewDecisionsRequired(r OutcomeReview, inForce []ItemDecision) bool {
	return len(OpenDecisions(r, inForce)) > 0
}
