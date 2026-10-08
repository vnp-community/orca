package domain

import (
	"reflect"
	"testing"
)

func TestStageForStatus(t *testing.T) {
	cr, _ := FlowFor(RequestTypeChangeRequest)
	bug, _ := FlowFor(RequestTypeBug)
	task, _ := FlowFor(RequestTypeTask)
	cases := []struct {
		status RequestStatus
		flow   FlowDefinition
		size   RequestSize
		want   []ReturnStage
	}{
		{RequestStatusClassifying, FlowDefinition{}, "", []ReturnStage{ReturnStageClassification}},
		{RequestStatusAwaitingTypeConfirmation, FlowDefinition{}, "", []ReturnStage{ReturnStageClassification}},
		{RequestStatusAnalyzing, bug, "", []ReturnStage{ReturnStageAnalysis}},
		{RequestStatusAwaitingAnalysisApproval, bug, "", []ReturnStage{ReturnStageAnalysis}},
		{RequestStatusPlanning, bug, "", []ReturnStage{ReturnStagePlan}},
		{RequestStatusAwaitingPlanApproval, bug, "", []ReturnStage{ReturnStagePlan}},
		{RequestStatusExecuting, task, RequestSizeL, []ReturnStage{ReturnStageTask}},
		{RequestStatusExecuting, bug, RequestSizeS, []ReturnStage{ReturnStageTask}},
		{RequestStatusExecuting, bug, RequestSizeL, []ReturnStage{ReturnStagePhase, ReturnStageTask}},
		{RequestStatusExecuting, cr, "", []ReturnStage{ReturnStagePhase, ReturnStageTask}},
	}
	for _, c := range cases {
		got, err := StageForStatus(c.status, c.flow, c.size)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s/%s/%q = %v, %v; want %v", c.status, c.flow.Type, c.size, got, err, c.want)
		}
	}
	for _, st := range []RequestStatus{RequestStatusNew, RequestStatusCompleted, RequestStatusRequestBacklog, RequestStatusCancelled} {
		if _, err := StageForStatus(st, cr, ""); errCode(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
			t.Errorf("%s: %v", st, err)
		}
	}
}

func TestParseReturnCategoryAndStage(t *testing.T) {
	for _, c := range AllReturnCategories() {
		if got, err := ParseReturnCategory(string(c)); err != nil || got != c {
			t.Errorf("category %s: %v", c, err)
		}
	}
	if _, err := ParseReturnCategory("nope"); errCode(err) != "REQUEST_RETURN_CATEGORY_INVALID" {
		t.Errorf("got %v", err)
	}
	for _, s := range AllReturnStages() {
		if got, err := ParseReturnStage(string(s)); err != nil || got != s {
			t.Errorf("stage %s: %v", s, err)
		}
	}
	if _, err := ParseReturnStage(""); errCode(err) != "REQUEST_RETURN_STAGE_INVALID" {
		t.Errorf("got %v", err)
	}
}
