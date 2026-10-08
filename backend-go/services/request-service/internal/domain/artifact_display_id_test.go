package domain

import "testing"

func TestDisplayID_FormatParse_RoundTrip(t *testing.T) {
	cases := []struct {
		s    string
		want DisplayID
	}{
		{FormatRequestID(142), DisplayID{Kind: DisplayKindRequest, ReqNum: 142}},
		{FormatAC(142, "AC-2"), DisplayID{Kind: DisplayKindAC, ReqNum: 142, Index: 2}},
		{FormatSolutionID(142, 2), DisplayID{Kind: DisplayKindSolution, ReqNum: 142, Seq: 2}},
		{FormatOptionID(FormatSolutionID(142, 2), 1), DisplayID{Kind: DisplayKindOption, ReqNum: 142, Seq: 2, Index: 1}},
		{FormatPlanID(142, 3), DisplayID{Kind: DisplayKindPlan, ReqNum: 142, Seq: 3}},
		{FormatPhaseID(142, 3, 2), DisplayID{Kind: DisplayKindPhase, ReqNum: 142, Seq: 3, Index: 2}},
		{FormatTaskID(142, 3, 7), DisplayID{Kind: DisplayKindTask, ReqNum: 142, Seq: 3, Index: 7}},
		{FormatClarificationID(142, 1), DisplayID{Kind: DisplayKindClarification, ReqNum: 142, Seq: 1}},
		{FormatDecisionID(142, 4), DisplayID{Kind: DisplayKindDecision, ReqNum: 142, Seq: 4}},
	}
	for _, c := range cases {
		got, err := ParseDisplayID(c.s)
		if err != nil || got != c.want {
			t.Errorf("%s: got %+v, %v want %+v", c.s, got, err, c.want)
		}
	}
	if FormatAC(142, "AC-2") != "REQ-142#AC-2" || FormatOptionID("SOL-142.2", 1) != "SOL-142.2/opt-1" {
		t.Error("documented forms changed")
	}
}

func TestParseDisplayID_RejectsMalformed(t *testing.T) {
	for _, s := range []string{"", "req-142", "REQ-0", "REQ-01", " REQ-142", "REQ-142 ", "SOL-142", "SOL-142.", "SOL-142.2/opt-", "SOL-142.2/opt-0",
		"SOL-142.2/OPT-1", "PH-1.2", "TSK-1.2.3.4", "AC-3", "REQ-142#AC-0", "REQ-142#ac-1", "REQ--1", "SOL-1.1\n"} {
		if _, err := ParseDisplayID(s); errCode(err) != "REQUEST_ARTIFACT_ID_INVALID" {
			t.Errorf("%q: got %v", s, err)
		}
	}
}
