package domain

import (
	"reflect"
	"testing"
)

type pair struct {
	from RequestStatus
	trig Trigger
}

func flowOf(t *testing.T, typ RequestType) FlowDefinition {
	t.Helper()
	f, err := FlowFor(typ)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// validPairs is the hand-written copy of CR-REQ-003 section 2.2 (from, trigger) -> to for a flow
// with analysis, an analysis gate and a plan (change_request); to is filled per flow below.
func validPairs() map[pair]bool {
	m := map[pair]bool{
		{RequestStatusNew, TriggerStartClassification}:                   true,
		{RequestStatusClassifying, TriggerProposalReady}:                 true,
		{RequestStatusAwaitingTypeConfirmation, TriggerTypeConfirmed}:    true,
		{RequestStatusAnalyzing, TriggerAnalysisReady}:                   true,
		{RequestStatusAwaitingAnalysisApproval, TriggerAnalysisApproved}: true,
		{RequestStatusAwaitingAnalysisApproval, TriggerAnalysisRejected}: true,
		{RequestStatusAwaitingAnalysisApproval, TriggerAnalysisRevision}: true,
		{RequestStatusPlanning, TriggerPlanReady}:                        true,
		{RequestStatusAwaitingPlanApproval, TriggerPlanApproved}:         true,
		{RequestStatusAwaitingPlanApproval, TriggerPlanRejected}:         true,
		{RequestStatusAwaitingPlanApproval, TriggerPlanRevision}:         true,
		{RequestStatusExecuting, TriggerExecutionFinished}:               true,
		{RequestStatusRequestBacklog, TriggerReopen}:                     true,
	}
	for _, st := range []RequestStatus{RequestStatusClassifying, RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing,
		RequestStatusAwaitingAnalysisApproval, RequestStatusPlanning, RequestStatusAwaitingPlanApproval, RequestStatusExecuting,
		RequestStatusAwaitingInformation} {
		m[pair{st, TriggerReturnToBacklog}] = true
	}
	for _, st := range []RequestStatus{RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval,
		RequestStatusPlanning, RequestStatusAwaitingPlanApproval, RequestStatusExecuting, RequestStatusAwaitingInformation} {
		m[pair{st, TriggerTypeChange}] = true
	}
	// CR-REQ-028: six statuses may open a Clarification; information_provided needs a resume target
	// and is therefore covered by NextStatusWithResume, not by this table.
	for _, st := range []RequestStatus{RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing, RequestStatusAwaitingAnalysisApproval,
		RequestStatusPlanning, RequestStatusAwaitingPlanApproval, RequestStatusExecuting} {
		m[pair{st, TriggerInformationRequired}] = true
	}
	for _, st := range AllRequestStatuses() {
		if st != RequestStatusCompleted && st != RequestStatusCancelled {
			m[pair{st, TriggerCancel}] = true
		}
	}
	return m
}

// TestNextStatus_FullMatrix covers the 12 statuses x 18 triggers matrix.
func TestNextStatus_FullMatrix(t *testing.T) {
	valid := validPairs()
	// 13 base + 8 return_to_backlog + 7 type_change + 10 cancel-able statuses + 6 information_required.
	if len(valid) != 13+8+7+10+6 {
		t.Fatalf("hand-written table has %d pairs", len(valid))
	}
	for _, typ := range []RequestType{RequestTypeChangeRequest, RequestTypeHotfix, RequestTypeSpike, RequestTypeTask} {
		flow := flowOf(t, typ)
		for _, from := range AllRequestStatuses() {
			for _, trig := range AllTriggers() {
				to, err := NextStatus(flow, "", from, trig)
				if valid[pair{from, trig}] {
					if err != nil {
						t.Errorf("%s %s --%s--> unexpected error %v", typ, from, trig, err)
					} else if to == "" {
						t.Errorf("%s %s --%s--> empty status", typ, from, trig)
					}
					continue
				}
				if errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
					t.Errorf("%s %s --%s--> want NOT_ALLOWED, got to=%q err=%v", typ, from, trig, to, err)
				}
			}
		}
	}
}

func TestTerminalStatesAcceptNothing(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for _, from := range []RequestStatus{RequestStatusCompleted, RequestStatusCancelled} {
		for _, trig := range AllTriggers() {
			if _, err := NextStatus(flow, RequestSizeL, from, trig); errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
				t.Errorf("%s --%s--> %v", from, trig, err)
			}
		}
	}
}

func TestNextStatus_PerType(t *testing.T) {
	S := func(xs ...RequestStatus) []RequestStatus { return xs }
	st := struct {
		n, c, atc, an, aaa, pl, apa, ex, co RequestStatus
	}{"new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval", "planning", "awaiting_plan_approval", "executing", "completed"}
	want := map[RequestType][]RequestStatus{
		RequestTypeChangeRequest: S(st.n, st.c, st.atc, st.an, st.aaa, st.pl, st.apa, st.ex, st.co),
		RequestTypeBug:           S(st.n, st.c, st.atc, st.an, st.aaa, st.pl, st.apa, st.ex, st.co),
		RequestTypeHotfix:        S(st.n, st.c, st.atc, st.an, st.apa, st.ex, st.co),
		RequestTypeTask:          S(st.n, st.c, st.atc, st.pl, st.apa, st.ex, st.co),
		RequestTypeSpike:         S(st.n, st.c, st.atc, st.an, st.aaa, st.co),
		RequestTypeQuestion:      S(st.n, st.c, st.atc, st.an, st.aaa, st.co),
		RequestTypeRefactor:      S(st.n, st.c, st.atc, st.an, st.aaa, st.pl, st.apa, st.ex, st.co),
		RequestTypeSecurity:      S(st.n, st.c, st.atc, st.an, st.aaa, st.pl, st.apa, st.ex, st.co),
		RequestTypePerformance:   S(st.n, st.c, st.atc, st.an, st.aaa, st.pl, st.apa, st.ex, st.co),
		RequestTypeDocs:          S(st.n, st.c, st.atc, st.pl, st.apa, st.ex, st.co),
		RequestTypeOpsRequest:    S(st.n, st.c, st.atc, st.pl, st.apa, st.ex, st.co),
	}
	if len(want) != 11 {
		t.Fatalf("want 11 types, have %d", len(want))
	}
	for typ, path := range want {
		got, err := HappyPath(flowOf(t, typ), "")
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if !reflect.DeepEqual(got, path) {
			t.Errorf("%s:\n got  %v\n want %v", typ, got, path)
		}
	}
}

func TestHappyPath_AllTypes(t *testing.T) {
	for _, typ := range AllFlowTypes() {
		f := flowOf(t, typ)
		steps, err := HappyPathSteps(f, RequestSizeL)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		path, _ := HappyPath(f, RequestSizeL)
		if len(path) != len(steps)+1 || path[0] != RequestStatusNew || path[len(path)-1] != RequestStatusCompleted {
			t.Errorf("%s: bad path %v", typ, path)
		}
		// No status repeats: the standard path never loops.
		seen := map[RequestStatus]bool{}
		for _, s := range path {
			if seen[s] {
				t.Errorf("%s: %s repeats in %v", typ, s, path)
			}
			seen[s] = true
		}
	}
}

func TestHotfixSkipsAnalysisApprovalAndPlanning(t *testing.T) {
	path, _ := HappyPath(flowOf(t, RequestTypeHotfix), "")
	for _, s := range path {
		if s == RequestStatusAwaitingAnalysisApproval || s == RequestStatusPlanning {
			t.Fatalf("hotfix must not visit %s: %v", s, path)
		}
	}
}

func TestNextStatus_AnalysisRejectedAndPlanRejectedGoToBacklog(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for from, trig := range map[RequestStatus]Trigger{RequestStatusAwaitingAnalysisApproval: TriggerAnalysisRejected, RequestStatusAwaitingPlanApproval: TriggerPlanRejected} {
		to, err := NextStatus(flow, "", from, trig)
		if err != nil || to != RequestStatusRequestBacklog {
			t.Errorf("%s/%s = %s, %v", from, trig, to, err)
		}
	}
}

func TestNextStatus_TypeNotSetWithoutFlow(t *testing.T) {
	_, err := NextStatus(FlowDefinition{}, "", RequestStatusAwaitingTypeConfirmation, TriggerTypeConfirmed)
	if errCode(err) != "REQUEST_TYPE_NOT_SET" {
		t.Fatalf("got %v", err)
	}
	// A pair outside the table stays NOT_ALLOWED even without a flow.
	_, err = NextStatus(FlowDefinition{}, "", RequestStatusNew, TriggerTypeConfirmed)
	if errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
		t.Fatalf("got %v", err)
	}
	// Triggers that do not depend on the flow work for an untyped request.
	if to, err := NextStatus(FlowDefinition{}, "", RequestStatusClassifying, TriggerReturnToBacklog); err != nil || to != RequestStatusRequestBacklog {
		t.Fatalf("untyped return_to_backlog: %s %v", to, err)
	}
}

func TestRequiresReason(t *testing.T) {
	want := map[Trigger]bool{TriggerAnalysisRejected: true, TriggerPlanRejected: true, TriggerReturnToBacklog: true, TriggerCancel: true}
	for _, trig := range AllTriggers() {
		if RequiresReason(trig) != want[trig] {
			t.Errorf("RequiresReason(%s) = %v", trig, RequiresReason(trig))
		}
	}
	for _, trig := range AllTriggers() {
		if TargetsBacklog(trig) != (trig == TriggerAnalysisRejected || trig == TriggerPlanRejected || trig == TriggerReturnToBacklog) {
			t.Errorf("TargetsBacklog(%s)", trig)
		}
	}
}

func TestAllTriggersCount(t *testing.T) {
	if len(AllTriggers()) != 18 {
		t.Fatalf("want 18 triggers, got %d", len(AllTriggers()))
	}
}

func TestNextStatus_InformationRequiredLandsInAwaitingInformation(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for _, from := range informationSourceStatuses {
		to, err := NextStatus(flow, "", from, TriggerInformationRequired)
		if err != nil || to != RequestStatusAwaitingInformation {
			t.Errorf("%s: %s, %v", from, to, err)
		}
	}
}

func TestNextStatus_InformationProvidedNeedsResume(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	if _, err := NextStatus(flow, "", RequestStatusAwaitingInformation, TriggerInformationProvided); errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
		t.Fatalf("NextStatus must refuse information_provided, got %v", err)
	}
}

func TestNextStatusWithResume_OnlyThreeResumeValues(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for _, resume := range AllRequestStatuses() {
		to, err := NextStatusWithResume(flow, "", RequestStatusAwaitingInformation, TriggerInformationProvided, resume)
		if IsResumeStatus(resume) {
			if err != nil || to != resume {
				t.Errorf("resume %s: %s, %v", resume, to, err)
			}
			continue
		}
		if errCode(err) != "REQUEST_RESUME_STATUS_INVALID" {
			t.Errorf("resume %s: want REQUEST_RESUME_STATUS_INVALID, got %s, %v", resume, to, err)
		}
	}
}

func TestNextStatusWithResume_FromWrongState(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for _, from := range AllRequestStatuses() {
		if from == RequestStatusAwaitingInformation {
			continue
		}
		if _, err := NextStatusWithResume(flow, "", from, TriggerInformationProvided, RequestStatusAnalyzing); errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
			t.Errorf("%s: got %v", from, err)
		}
	}
}

func TestNextStatusWithResume_DelegatesOtherTriggers(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for _, from := range AllRequestStatuses() {
		for _, trig := range AllTriggers() {
			if trig == TriggerInformationProvided {
				continue
			}
			a, errA := NextStatus(flow, RequestSizeM, from, trig)
			b, errB := NextStatusWithResume(flow, RequestSizeM, from, trig, "")
			if a != b || errCode(errA) != errCode(errB) {
				t.Errorf("%s --%s--> NextStatus=%s/%v WithResume=%s/%v", from, trig, a, errA, b, errB)
			}
		}
	}
}

func TestAwaitingInformationExits(t *testing.T) {
	flow := flowOf(t, RequestTypeBug)
	for trig, want := range map[Trigger]RequestStatus{
		TriggerReturnToBacklog: RequestStatusRequestBacklog,
		TriggerTypeChange:      RequestStatusAwaitingTypeConfirmation,
		TriggerCancel:          RequestStatusCancelled,
	} {
		if to, err := NextStatus(flow, "", RequestStatusAwaitingInformation, trig); err != nil || to != want {
			t.Errorf("%s: %s, %v", trig, to, err)
		}
	}
}

func TestIsHumanWaiting(t *testing.T) {
	if !RequestStatusAwaitingInformation.IsHumanWaiting() || RequestStatusExecuting.IsHumanWaiting() {
		t.Fatal("awaiting_information waits for a person, executing does not")
	}
}
