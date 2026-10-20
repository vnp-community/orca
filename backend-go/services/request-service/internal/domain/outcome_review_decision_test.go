package domain

import (
	"strings"
	"testing"
)

func decisionReview() OutcomeReview {
	return OutcomeReview{
		Phase: ReviewPhaseDelivered, State: ReviewStateReady,
		Items: []ReviewItem{
			{Key: "need", Origin: OriginAgent, NeedsDecision: true, Title: "breaking"},
			{Key: "claim", Origin: OriginAgent, Title: "claim"},
			{Key: "fact", Origin: OriginFact, Title: "fact"},
			{Key: "need2", Origin: OriginFact, NeedsDecision: true, Title: "breaking fact"},
		},
	}
}

func TestCheckDecision_Rules(t *testing.T) {
	r := decisionReview()
	cases := []struct {
		name string
		item string
		d    ReviewDecision
		note string
		ok   bool
	}{
		{"accept a needed item with a note", "need", DecisionAcceptException, "ADR-09", true},
		{"accept a needed fact with a note", "need2", DecisionAcceptException, "known", true},
		{"accept without a note", "need", DecisionAcceptException, "   ", false},
		{"accept an item that asks for nothing", "claim", DecisionAcceptException, "x", false},
		{"dismiss an agent claim with a note", "claim", DecisionDismiss, "wrong", true},
		{"dismiss without a note", "claim", DecisionDismiss, "", false},
		{"dismiss a fact", "fact", DecisionDismiss, "no", false},
		{"dismiss a needed fact", "need2", DecisionDismiss, "no", false},
		{"send back a fact", "fact", DecisionSendBack, "", true},
		{"send back with a note", "need", DecisionSendBack, "please", true},
		{"a note of 500 characters", "claim", DecisionDismiss, strings.Repeat("é", 500), true},
		{"a note of 501 characters", "claim", DecisionDismiss, strings.Repeat("é", 501), false},
		{"an unknown item", "zzz", DecisionSendBack, "", false},
		{"an unknown decision", "claim", ReviewDecision("maybe"), "x", false},
	}
	for _, c := range cases {
		_, err := CheckDecision(r, c.item, c.d, c.note)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
	design := r
	design.Phase = ReviewPhaseDesign
	if _, err := CheckDecision(design, "need", DecisionSendBack, ""); err == nil || !strings.Contains(err.Error(), CodeReviewNotReady) {
		t.Errorf("a design review has no decisions: %v", err)
	}
	for _, st := range []ReviewState{ReviewStateStale, ReviewStateSuperseded} {
		x := r
		x.State = st
		if _, err := CheckDecision(x, "need", DecisionSendBack, ""); err == nil || !strings.Contains(err.Error(), CodeReviewStaleRevision) {
			t.Errorf("%s: %v", st, err)
		}
	}
}

func TestOpenDecisionsAndOrphans(t *testing.T) {
	r := decisionReview()
	if open := OpenDecisions(r, nil); len(open) != 2 {
		t.Fatalf("two items ask for a decision: %v", open)
	}
	inForce := []ItemDecision{{ItemKey: "need", Decision: DecisionSendBack, Active: true}, {ItemKey: "gone", Decision: DecisionDismiss, Active: true}, {ItemKey: "need2", Decision: DecisionAcceptException, Active: false}}
	open := OpenDecisions(r, inForce)
	if len(open) != 1 || open[0].Key != "need2" {
		t.Errorf("a send_back counts as decided, a replaced decision does not: %v", open)
	}
	if NewDecisionsRequired(r, inForce) != true || NewDecisionsRequired(r, append(inForce, ItemDecision{ItemKey: "need2", Decision: DecisionSendBack, Active: true})) {
		t.Error("new decisions are required exactly while something is open")
	}
	marked := MarkOrphans(r, inForce)
	if marked[0].Orphaned || !marked[1].Orphaned {
		t.Errorf("only a decision about an item that is gone is an orphan: %+v", marked)
	}
	if inForce[1].Orphaned {
		t.Error("the input is not changed")
	}
}

// A review that was sent back is handed to a child Request even when the Request is finished: the escalation link allows any parent that is not cancelled.
func TestSendBackCanSpawnAChildFromACompletedRequest(t *testing.T) {
	completed := Request{Status: RequestStatusCompleted, Type: RequestTypeChangeRequest}
	if err := ValidateChild(completed, LinkReasonEscalation, RequestTypeBug); err != nil {
		t.Errorf("escalation from a completed request: %v", err)
	}
	if err := ValidateChild(completed, LinkReasonEscalation, ""); err != nil {
		t.Errorf("escalation without a type hint: %v", err)
	}
	if err := ValidateChild(Request{Status: RequestStatusCancelled}, LinkReasonEscalation, ""); err == nil {
		t.Error("a cancelled request spawns nothing")
	}
}
