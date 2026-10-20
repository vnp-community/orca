package domain

import "testing"

func bptr(b bool) *bool { return &b }

func TestDeriveState_ThreeWayComparison(t *testing.T) {
	cases := []struct {
		name   string
		p      Presence
		state  EdgeState
		change EdgeState
		wantOK bool
	}{
		{"new edge with no design", Presence{InHead: true}, StateAdded, StateAdded, true},
		{"removed edge with no design", Presence{InBase: true}, StateRemoved, StateRemoved, true},
		{"unchanged edge", Presence{InBase: true, InHead: true}, StateSame, StateSame, true},
		{"new edge the design allows", Presence{InHead: true, InDesign: bptr(true)}, StateAdded, StateAdded, true},
		{"new edge the design does not have", Presence{InHead: true, InDesign: bptr(false)}, StateDeviation, StateAdded, true},
		{"old edge that is not in the design", Presence{InBase: true, InHead: true, InDesign: bptr(false)}, StateDeviation, StateSame, true},
		{"designed edge that was never built", Presence{InDesign: bptr(true)}, StateMissing, "", true},
		{"designed edge that was removed", Presence{InBase: true, InDesign: bptr(true)}, StateMissing, StateRemoved, true},
		{"designed edge present", Presence{InBase: true, InHead: true, InDesign: bptr(true)}, StateSame, StateSame, true},
		{"seen nowhere", Presence{}, "", "", false},
		{"not in code, design says no", Presence{InDesign: bptr(false)}, "", "", false},
	}
	for _, c := range cases {
		state, change, ok := DeriveState(c.p)
		if ok != c.wantOK || state != c.state || change != c.change {
			t.Errorf("%s: got (%q,%q,%v), want (%q,%q,%v)", c.name, state, change, ok, c.state, c.change, c.wantOK)
		}
	}
}

func TestDeriveState_NoDesignNeverGivesDeviationOrMissing(t *testing.T) {
	for _, base := range []bool{false, true} {
		for _, head := range []bool{false, true} {
			if s, _, ok := DeriveState(Presence{InBase: base, InHead: head}); ok && (s == StateDeviation || s == StateMissing) {
				t.Errorf("base=%v head=%v: %s without a design", base, head, s)
			}
		}
	}
}

func TestAdvisoryFor(t *testing.T) {
	if !AdvisoryFor(StateDeviation, KGApproved, KGNeedsReview) || !AdvisoryFor(StateMissing, KGUnknown) || !AdvisoryFor(StateDeviation) {
		t.Error("a deviation resting on an element that is not approved is advisory, and so is one with no basis")
	}
	if AdvisoryFor(StateDeviation, KGApproved, KGApproved) {
		t.Error("a deviation on approved elements asks for a decision")
	}
	if AdvisoryFor(StateAdded, KGNeedsReview) {
		t.Error("only deviation and missing can be advisory")
	}
}
