package domain

import "testing"

func TestValidateChild_Matrix(t *testing.T) {
	type tc struct {
		name   string
		parent Request
		reason LinkReason
		hint   RequestType
		ok     bool
	}
	p := func(typ RequestType, st RequestStatus) Request { return Request{Type: typ, Status: st} }
	cases := []tc{
		{"spike awaiting approval to change_request", p(RequestTypeSpike, RequestStatusAwaitingAnalysisApproval), LinkReasonSpawnedBySpike, RequestTypeChangeRequest, true},
		{"spike completed to task", p(RequestTypeSpike, RequestStatusCompleted), LinkReasonSpawnedBySpike, RequestTypeTask, true},
		{"spike with no hint", p(RequestTypeSpike, RequestStatusCompleted), LinkReasonSpawnedBySpike, "", true},
		{"spike to bug rejected", p(RequestTypeSpike, RequestStatusCompleted), LinkReasonSpawnedBySpike, RequestTypeBug, false},
		{"spike still analyzing rejected", p(RequestTypeSpike, RequestStatusAnalyzing), LinkReasonSpawnedBySpike, RequestTypeTask, false},
		{"question parent for spike reason rejected", p(RequestTypeQuestion, RequestStatusCompleted), LinkReasonSpawnedBySpike, RequestTypeTask, false},
		{"question completed to change_request", p(RequestTypeQuestion, RequestStatusCompleted), LinkReasonSpawnedByQuestion, RequestTypeChangeRequest, true},
		{"question to refactor rejected", p(RequestTypeQuestion, RequestStatusCompleted), LinkReasonSpawnedByQuestion, RequestTypeRefactor, false},
		{"hotfix executing to bug", p(RequestTypeHotfix, RequestStatusExecuting), LinkReasonFollowupHotfix, RequestTypeBug, true},
		{"hotfix completed to task", p(RequestTypeHotfix, RequestStatusCompleted), LinkReasonFollowupHotfix, RequestTypeTask, true},
		{"hotfix planning rejected", p(RequestTypeHotfix, RequestStatusAwaitingPlanApproval), LinkReasonFollowupHotfix, RequestTypeBug, false},
		{"hotfix to change_request rejected", p(RequestTypeHotfix, RequestStatusExecuting), LinkReasonFollowupHotfix, RequestTypeChangeRequest, false},
		{"bug parent for hotfix reason rejected", p(RequestTypeBug, RequestStatusExecuting), LinkReasonFollowupHotfix, RequestTypeBug, false},
		{"escalation from bug analyzing", p(RequestTypeBug, RequestStatusAnalyzing), LinkReasonEscalation, RequestTypeChangeRequest, true},
		{"escalation from backlog no hint", p(RequestTypeTask, RequestStatusRequestBacklog), LinkReasonEscalation, "", true},
		{"escalation from completed", p(RequestTypeDocs, RequestStatusCompleted), LinkReasonEscalation, "", true},
		{"escalation from cancelled rejected", p(RequestTypeBug, RequestStatusCancelled), LinkReasonEscalation, "", false},
		{"escalation with unknown hint rejected", p(RequestTypeBug, RequestStatusAnalyzing), LinkReasonEscalation, "feature", false},
		{"non spawn reason rejected", p(RequestTypeSpike, RequestStatusCompleted), LinkReasonBlocks, "", false},
	}
	for _, c := range cases {
		err := ValidateChild(c.parent, c.reason, c.hint)
		if c.ok && err != nil {
			t.Errorf("%s: unexpected %v", c.name, err)
		}
		if !c.ok && (err == nil || (errCode(err) != "REQUEST_CHILD_NOT_ALLOWED" && errCode(err) != "REQUEST_INVALID_TYPE")) {
			t.Errorf("%s: want rejection, got %v", c.name, err)
		}
	}
}

func TestParseLinkReason(t *testing.T) {
	for _, r := range []LinkReason{LinkReasonRelatesTo, LinkReasonSpawnedBySpike, LinkReasonEscalation} {
		if _, err := ParseLinkReason(string(r)); err != nil {
			t.Errorf("%s: %v", r, err)
		}
	}
	if _, err := ParseLinkReason("bogus"); err == nil {
		t.Error("bogus accepted")
	}
}
