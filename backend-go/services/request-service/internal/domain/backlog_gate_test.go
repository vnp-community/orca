package domain

import (
	"fmt"
	"testing"
	"time"
)

var gateEpoch = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func appr(st SubjectType, id string, status ApprovalStatus, minutes int) Approval {
	return Approval{ID: fmt.Sprintf("a-%s-%s-%d", st, id, minutes), SubjectType: st, SubjectID: id, Status: status, CreatedAt: gateEpoch.Add(time.Duration(minutes) * time.Minute)}
}

func gateInput(typ RequestType, size RequestSize, container, plan *TaskView, approvals ...Approval) TaskGateInput {
	return TaskGateInput{
		Request: Request{Type: typ, Size: size}, Task: TaskView{ID: "task-1", Type: "task"},
		Container: container, Plan: plan, Approvals: NewApprovalIndex(approvals),
	}
}

var (
	gatePlan  = &TaskView{ID: "plan-1", Type: TaskTypePlan}
	gatePhase = &TaskView{ID: "phase-1", ParentID: "plan-1", Type: TaskTypePhase}
)

func TestResolveTaskGate_AllTypes_Table(t *testing.T) {
	approved := func(st SubjectType, id string) Approval { return appr(st, id, ApprovalStatusApproved, 1) }
	pending := func(st SubjectType, id string) Approval { return appr(st, id, ApprovalStatusPending, 1) }
	cases := []struct {
		name  string
		input TaskGateInput
		want  GateStatus
		split bool
	}{
		// change_request: Phase gate plus Plan gate
		{"change_request phase+plan approved", gateInput(RequestTypeChangeRequest, RequestSizeM, gatePhase, gatePlan, approved(SubjectPhase, "phase-1"), approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"change_request plan approved, phase pending", gateInput(RequestTypeChangeRequest, RequestSizeM, gatePhase, gatePlan, pending(SubjectPhase, "phase-1"), approved(SubjectPlan, "plan-1")), GateStatusPending, false},
		{"change_request plan approved, phase never opened", gateInput(RequestTypeChangeRequest, RequestSizeM, gatePhase, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusNone, false},
		{"change_request phase approved, plan pending", gateInput(RequestTypeChangeRequest, RequestSizeM, gatePhase, gatePlan, approved(SubjectPhase, "phase-1"), pending(SubjectPlan, "plan-1")), GateStatusPending, false},
		{"change_request task straight under plan waits for split", gateInput(RequestTypeChangeRequest, RequestSizeS, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusNone, true},
		// bug and refactor: phases only at size L, no phase gate
		{"bug L phase: plan approved is enough", gateInput(RequestTypeBug, RequestSizeL, gatePhase, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"bug L phase: plan pending", gateInput(RequestTypeBug, RequestSizeL, gatePhase, gatePlan, pending(SubjectPlan, "plan-1")), GateStatusPending, false},
		{"bug L straight under plan: waiting for split", gateInput(RequestTypeBug, RequestSizeL, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusNone, true},
		{"bug S under plan approved", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"bug S under plan pending", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, pending(SubjectPlan, "plan-1")), GateStatusPending, false},
		{"refactor L phase approved via plan", gateInput(RequestTypeRefactor, RequestSizeL, gatePhase, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"refactor M under plan approved", gateInput(RequestTypeRefactor, RequestSizeM, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		// task and docs: task_list shell
		{"task under task_list approved", gateInput(RequestTypeTask, RequestSizeS, gatePlan, gatePlan, approved(SubjectTaskList, "plan-1")), GateStatusApproved, false},
		{"task under task_list pending", gateInput(RequestTypeTask, RequestSizeS, gatePlan, gatePlan, pending(SubjectTaskList, "plan-1")), GateStatusPending, false},
		{"task: a plan approval is not a task_list approval", gateInput(RequestTypeTask, RequestSizeS, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusNone, false},
		{"docs under task_list approved", gateInput(RequestTypeDocs, RequestSizeM, gatePlan, gatePlan, approved(SubjectTaskList, "plan-1")), GateStatusApproved, false},
		{"docs L never splits into phases", gateInput(RequestTypeDocs, RequestSizeL, gatePlan, gatePlan, approved(SubjectTaskList, "plan-1")), GateStatusApproved, false},
		// hotfix: pre_deploy on the single task
		{"hotfix after pre_deploy", gateInput(RequestTypeHotfix, RequestSizeS, nil, nil, approved(SubjectPreDeploy, "task-1")), GateStatusApproved, false},
		{"hotfix pre_deploy pending", gateInput(RequestTypeHotfix, RequestSizeS, nil, nil, pending(SubjectPreDeploy, "task-1")), GateStatusPending, false},
		{"hotfix no approval yet", gateInput(RequestTypeHotfix, RequestSizeS, nil, nil), GateStatusNone, false},
		// security: pre_deploy on the plan
		{"security plan pre_deploy approved", gateInput(RequestTypeSecurity, RequestSizeM, gatePlan, gatePlan, approved(SubjectPreDeploy, "plan-1")), GateStatusApproved, false},
		{"security with a plan approval only", gateInput(RequestTypeSecurity, RequestSizeM, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusNone, false},
		// performance and ops_request: plan gate (ops adds per-task gates at dispatch time)
		{"performance plan approved", gateInput(RequestTypePerformance, RequestSizeM, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"ops_request plan approved", gateInput(RequestTypeOpsRequest, RequestSizeM, gatePlan, gatePlan, approved(SubjectPlan, "plan-1")), GateStatusApproved, false},
		{"ops_request plan pending", gateInput(RequestTypeOpsRequest, RequestSizeM, gatePlan, gatePlan, pending(SubjectPlan, "plan-1")), GateStatusPending, false},
		// re-approval: a newer pending or rejected row overrides an older approved one
		{"newer pending after approved", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, appr(SubjectPlan, "plan-1", ApprovalStatusApproved, 1), appr(SubjectPlan, "plan-1", ApprovalStatusPending, 5)), GateStatusPending, false},
		{"newest rejected", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, appr(SubjectPlan, "plan-1", ApprovalStatusApproved, 1), appr(SubjectPlan, "plan-1", ApprovalStatusRejected, 9)), GateStatusRejected, false},
		{"newest approved after rejected", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, appr(SubjectPlan, "plan-1", ApprovalStatusRejected, 1), appr(SubjectPlan, "plan-1", ApprovalStatusApproved, 9)), GateStatusApproved, false},
		{"expired approval is not approved", gateInput(RequestTypeBug, RequestSizeS, gatePlan, gatePlan, appr(SubjectPlan, "plan-1", ApprovalStatusExpired, 1)), GateStatusNone, false},
		// spike and question have no execution at all
		{"spike has no plan gate", gateInput(RequestTypeSpike, RequestSizeS, gatePlan, gatePlan), GateStatusNone, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveTaskGate(tc.input)
			if got.Status != tc.want || got.WaitingForPhaseSplit != tc.split || got.Approved != (tc.want == GateStatusApproved) {
				t.Fatalf("got %+v, want status=%s split=%v", got, tc.want, tc.split)
			}
		})
	}
}

func TestResolveTaskGate_EveryTypeHasAnApprovedAndAnUnapprovedCase(t *testing.T) {
	for _, typ := range AllFlowTypes() {
		flow, _ := FlowFor(typ)
		subject, ok := startGateSubject(flow.StartGate)
		if flow.PlanKind == PlanNone {
			continue
		}
		if flow.PlanKind == PlanSingleTask {
			subject, ok = SubjectPreDeploy, true
		}
		if !ok {
			t.Errorf("%s: no start gate subject", typ)
			continue
		}
		size := RequestSizeS
		var container, plan *TaskView = gatePlan, gatePlan
		subjectID := "plan-1"
		if flow.PlanKind == PlanSingleTask {
			container, plan, subjectID = nil, nil, "task-1"
		}
		unapproved := ResolveTaskGate(gateInput(typ, size, container, plan))
		approved := ResolveTaskGate(gateInput(typ, size, container, plan, appr(subject, subjectID, ApprovalStatusApproved, 1)))
		if flow.PhasesFor(size) {
			continue
		}
		if unapproved.Approved || !approved.Approved {
			t.Errorf("%s: unapproved=%+v approved=%+v", typ, unapproved, approved)
		}
	}
}

func TestResolveTaskGate_UntypedRequestDoesNotPanic(t *testing.T) {
	res := ResolveTaskGate(TaskGateInput{Request: Request{}, Task: TaskView{ID: "t"}, Container: gatePlan, Plan: gatePlan})
	if res.WaitingForPhaseSplit || res.Approved {
		t.Fatalf("untyped request must wait for an approval, got %+v", res)
	}
}

func TestApprovalIndex_LatestWins_TieBreak(t *testing.T) {
	older := appr(SubjectPlan, "p", ApprovalStatusApproved, 1)
	newer := appr(SubjectPlan, "p", ApprovalStatusPending, 2)
	idx := NewApprovalIndex([]Approval{newer, older})
	if got := idx.GetLatest(SubjectPlan, "p"); got == nil || got.Status != ApprovalStatusPending {
		t.Fatalf("newest by created_at must win, got %+v", got)
	}
	a := Approval{ID: "a", SubjectType: SubjectPlan, SubjectID: "p", Status: ApprovalStatusApproved, CreatedAt: gateEpoch}
	b := Approval{ID: "b", SubjectType: SubjectPlan, SubjectID: "p", Status: ApprovalStatusRejected, CreatedAt: gateEpoch}
	for _, in := range [][]Approval{{a, b}, {b, a}} {
		if got := NewApprovalIndex(in).GetLatest(SubjectPlan, "p"); got == nil || got.ID != "b" {
			t.Fatalf("equal created_at must pick the larger id regardless of input order, got %+v", got)
		}
	}
	if NewApprovalIndex(nil).GetLatest(SubjectPlan, "x") != nil {
		t.Fatal("empty index must return nil")
	}
}

func TestPageToken_RoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 8, 12, 30, 45, 123456000, time.UTC)
	gotAt, gotID, err := DecodePageToken(EncodePageToken(at, "req-9"))
	if err != nil || !gotAt.Equal(at) || gotID != "req-9" {
		t.Fatalf("round trip: %v %v %v", gotAt, gotID, err)
	}
}

func TestPageToken_Bad(t *testing.T) {
	for name, tok := range map[string]string{
		"bad base64": "!!!", "bad json": "bm90LWpzb24", "empty id": EncodePageToken(gateEpoch, ""), "zero time": EncodePageToken(time.Time{}, "x"),
	} {
		if _, _, err := DecodePageToken(tok); err != ErrBadPageToken {
			t.Errorf("%s: got %v, want ErrBadPageToken", name, err)
		}
	}
	if at, id, err := DecodePageToken(""); err != nil || !at.IsZero() || id != "" {
		t.Errorf("empty token means first page, got %v %q %v", at, id, err)
	}
}
