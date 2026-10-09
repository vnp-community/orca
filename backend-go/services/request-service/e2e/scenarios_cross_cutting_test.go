//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
)

// crossCuttingScenarios are E13 to E16 and E19 of CR-REQ-025 section 2.3. E17 (MCP) and E18 (Jira) need the
// dev stack and live in tests/request (T2); E20 (the flag) is feature_flag_test.go.
func crossCuttingScenarios() []Scenario {
	return []Scenario{e13ReturnAndReopen(), e14ChangeTypeMidFlight(), e15TaskFailureReturnsToBacklog(), e16OutcomeCallbackIdempotent(), e19NotAnApprover()}
}

// startConfirmed brings a Request to the first stage after type confirmation.
func startConfirmed(typ domain.RequestType, size domain.RequestSize) []Stage {
	return []Stage{
		{
			Name:       "create_and_classify",
			Run:        func(t *testing.T, w *world) { w.request = w.k.create("cross-cutting", w.typ, w.size) },
			WantStatus: "awaiting_type_confirmation",
		},
		{
			Name: "human_confirms_type",
			Run: func(t *testing.T, w *world) {
				w.request = w.k.confirm(w.request, w.typ, w.size)
			},
			WantStatus: "analyzing",
		},
	}
}

// E13: a Request returned to the backlog can be reopened and is classified again, and a rejected Plan sends it back
// to the backlog with the reviewer's reason.
func e13ReturnAndReopen() Scenario {
	sc := Scenario{ID: "E13", Group: GroupCrossCut, Type: string(domain.RequestTypeBug), Size: "M", Stages: startConfirmed(domain.RequestTypeBug, domain.RequestSizeM)}
	sc.Stages = append(sc.Stages,
		Stage{
			Name: "return_to_backlog_from_analysis",
			Run: func(t *testing.T, w *world) {
				resp, err := w.k.req.ReturnToBacklog(w.k.asReporter(), &requestv1.ReturnToBacklogRequest{
					RequestId: w.request.GetId(), Stage: "analysis", Category: "missing_info", Reason: "needs the failing log"})
				w.callOK(err, "ReturnToBacklog")
				if resp.GetRequest().GetReturnedFromStage() != "analysis" {
					t.Fatalf("returned_from_stage = %q", resp.GetRequest().GetReturnedFromStage())
				}
			},
			WantStatus: "request_backlog",
			WantEvents: []string{domain.SubjectRequestReturned},
			WantAudits: []string{domain.ActionRequestReturn},
		},
		Stage{
			Name: "reopen_and_reclassify",
			Run: func(t *testing.T, w *world) {
				_, err := w.k.req.ReopenRequest(w.k.asReporter(), &requestv1.ReopenRequestRequest{RequestId: w.request.GetId(), Note: "log attached"})
				w.callOK(err, "ReopenRequest")
			},
			// classifying is passed through on the way: the status_changed consumer proposes again.
			WantStatus: "awaiting_type_confirmation",
			WantAudits: []string{domain.ActionRequestReopen},
		},
		Stage{
			Name:       "confirm_type_again",
			Run:        func(t *testing.T, w *world) { w.request = w.k.confirm(w.request, w.typ, w.size) },
			WantStatus: "analyzing",
		},
		Stage{Name: "analyzing_to_awaiting_analysis_approval", Run: func(t *testing.T, w *world) { w.analyze() }, WantStatus: "awaiting_analysis_approval", WantApprovals: []string{"solution"}},
		Stage{Name: "awaiting_analysis_approval_to_planning", Run: func(t *testing.T, w *world) { w.approveAnalysis() }, WantStatus: "planning"},
		Stage{Name: "planning_to_awaiting_plan_approval", Run: func(t *testing.T, w *world) { w.plan() }, WantStatus: "awaiting_plan_approval", WantApprovals: []string{"plan"}},
		Stage{
			Name: "plan_rejection_returns_to_backlog",
			Run: func(t *testing.T, w *world) {
				a := w.pendingApproval("plan", "plan")
				_, err := w.k.appr.Reject(w.k.asAdmin(), &requestv1.RejectRequest{Id: a.GetId(), Comment: "split the work differently", ExpectedDigest: a.GetSubjectDigest()})
				w.callOK(err, "Reject")
			},
			WantStatus: "request_backlog",
			WantEvents: []string{domain.SubjectRequestReturned},
		},
		Stage{
			Name: "rejected_plan_is_recorded_on_the_request",
			Run: func(t *testing.T, w *world) {
				got := w.k.get(w.id())
				if got.GetReturnedFromStage() != "plan" || got.GetReturnedCategory() != "rejected" || got.GetReturnReason() != "split the work differently" {
					t.Fatalf("returned from %q as %q because %q", got.GetReturnedFromStage(), got.GetReturnedCategory(), got.GetReturnReason())
				}
				w.request = got
			},
		},
	)
	return sc
}

// E14: the type changes while the Request is in flight; the history keeps both types and the Request goes back to
// type confirmation.
func e14ChangeTypeMidFlight() Scenario {
	sc := Scenario{ID: "E14", Group: GroupCrossCut, Type: string(domain.RequestTypeBug), Size: "M", Stages: startConfirmed(domain.RequestTypeBug, domain.RequestSizeM)}
	sc.Stages = append(sc.Stages, Stage{
		Name: "change_type_to_change_request",
		Run: func(t *testing.T, w *world) {
			_, err := w.k.req.ChangeRequestType(w.k.asReporter(), &requestv1.ChangeRequestTypeRequest{
				RequestId: w.request.GetId(), NewType: "change_request", Size: "M", Reason: "needs a design, not a patch"})
			w.callOK(err, "ChangeRequestType")
			hist, err := w.k.req.ListRequestTypeHistory(w.k.asReporter(), &requestv1.ListRequestTypeHistoryRequest{RequestId: w.request.GetId()})
			w.callOK(err, "ListRequestTypeHistory")
			var pairs []string
			for _, h := range hist.GetChanges() {
				pairs = append(pairs, h.GetFromType()+">"+h.GetToType())
			}
			if !contains(pairs, "bug>change_request") {
				t.Fatalf("type history lacks bug>change_request: %v", pairs)
			}
		},
		WantStatus: "awaiting_type_confirmation",
		WantEvents: []string{domain.SubjectRequestTypeChanged},
		WantAudits: []string{domain.ActionRequestTypeChange},
	})
	return sc
}

// toExecuting brings a type without analysis from the first stage to executing.
func toExecuting(typ domain.RequestType, size domain.RequestSize) []Stage {
	return append(startConfirmedNoAnalysis(typ, size),
		Stage{Name: "planning_to_awaiting_plan_approval", Run: func(t *testing.T, w *world) { w.plan() }, WantStatus: "awaiting_plan_approval", WantApprovals: []string{"task_list"}},
		Stage{Name: "awaiting_plan_approval_to_executing", Run: func(t *testing.T, w *world) { w.approvePlan() }, WantStatus: "executing"},
	)
}

// dispatched waits until task-service was asked to run the Request's only working task for the given attempt.
func (w *world) dispatched(attempt int) string {
	k := w.k
	var taskID string
	eventually(k.t, 30*time.Second, fmt.Sprintf("attempt %d to be dispatched", attempt), func() (bool, string) {
		for _, ref := range theStack.tasks.Executed() {
			parts := strings.Split(ref, ":") // req:<request>:<task>:<attempt>
			if len(parts) == 4 && parts[1] == w.id() && parts[3] == fmt.Sprint(attempt) {
				taskID = parts[2]
				return true, ""
			}
		}
		return false, fmt.Sprintf("executed so far: %v", theStack.tasks.Executed())
	})
	return taskID
}

// E15: a task that fails every attempt sends the Request back to the backlog with the last error.
func e15TaskFailureReturnsToBacklog() Scenario {
	sc := Scenario{ID: "E15", Group: GroupCrossCut, Type: string(domain.RequestTypeTask), Size: "S", Stages: toExecuting(domain.RequestTypeTask, domain.RequestSizeS)}
	sc.Stages = append(sc.Stages, Stage{
		Name: "failed_task_returns_request_to_backlog",
		Run: func(t *testing.T, w *world) {
			task := w.dispatched(1)
			theStack.tasks.RunFailed(task, "compile error 1")
			if again := w.dispatched(2); again != task {
				t.Fatalf("the second attempt ran task %s, want %s again", again, task)
			}
			if got := w.k.get(w.id()); got.GetStatus() != "executing" {
				t.Fatalf("one failure is not enough to give up: %s", got.GetStatus())
			}
			theStack.tasks.RunFailed(task, "compile error 2")
		},
		WantStatus: "request_backlog",
		WantEvents: []string{domain.SubjectRequestReturned},
	}, Stage{
		Name: "the_backlog_entry_names_the_task_and_its_last_error",
		Run: func(t *testing.T, w *world) {
			got := w.k.get(w.id())
			if got.GetReturnedFromStage() != "task" || !strings.Contains(got.GetReturnReason(), "compile error 2") || !strings.Contains(got.GetReturnReason(), "Do the work") {
				t.Fatalf("returned from %q because %q", got.GetReturnedFromStage(), got.GetReturnReason())
			}
		},
	})
	return sc
}

// E16: a task event delivered twice, or after the Request moved on, changes nothing.
func e16OutcomeCallbackIdempotent() Scenario {
	sc := Scenario{ID: "E16", Group: GroupCrossCut, Type: string(domain.RequestTypeTask), Size: "S", Stages: toExecuting(domain.RequestTypeTask, domain.RequestSizeS)}
	sc.Stages = append(sc.Stages,
		Stage{
			Name: "a_repeated_start_event_dispatches_nothing_new",
			Run: func(t *testing.T, w *world) {
				w.dispatched(1)
				first := theStack.tasks.eventsOf(w.id())
				if len(first) == 0 {
					t.Fatal("the dispatch produced no task event")
				}
				for i := 0; i < 2; i++ {
					w.replay(first[0])
				}
				time.Sleep(500 * time.Millisecond) // a wrongly started second run would be dispatched within this time
				if n := len(w.executions()); n != 1 {
					t.Fatalf("task-service was asked to run the task %d times: %v", n, w.executions())
				}
			},
			WantStatus: "executing",
		},
		Stage{
			Name:          "the_run_finishes_and_the_request_completes",
			Run:           func(t *testing.T, w *world) { theStack.tasks.RunSucceeded(w.dispatched(1)) },
			WantStatus:    "completed",
			WantEvents:    []string{domain.SubjectRequestCompleted},
			StatusTimeout: 60 * time.Second,
		},
		Stage{
			Name: "reordered_and_duplicate_events_after_completion_change_nothing",
			Run: func(t *testing.T, w *world) {
				events := theStack.tasks.eventsOf(w.id())
				for i := len(events) - 1; i >= 0; i-- { // newest first: the order task-service never sends
					w.replay(events[i])
					w.replay(events[i])
				}
				time.Sleep(500 * time.Millisecond)
				if got := w.k.get(w.id()); got.GetStatus() != "completed" {
					t.Fatalf("replayed events moved a completed Request to %s", got.GetStatus())
				}
				if n := len(w.k.outbox(domain.SubjectRequestCompleted)); n != 1 {
					t.Fatalf("request.completed was emitted %d times", n)
				}
				if n := len(w.executions()); n != 1 {
					t.Fatalf("a replayed event started another run: %v", w.executions())
				}
			},
		},
	)
	return sc
}

// replay sends a task event to the service again, as a redelivery would.
func (w *world) replay(ev *requestv1.ReportTaskOutcomeRequest) {
	_, err := w.k.req.ReportTaskOutcome(w.k.as(w.k.reporter, "user"), ev)
	w.callOK(err, "ReportTaskOutcome (replay)")
}

// executions lists the runs task-service was asked to start for this Request.
func (w *world) executions() []string {
	var out []string
	for _, ref := range theStack.tasks.Executed() {
		if strings.HasPrefix(ref, "req:"+w.id()+":") {
			out = append(out, ref)
		}
	}
	return out
}

// startConfirmedNoAnalysis is startConfirmed for types with no analysis stage (they land in planning).
func startConfirmedNoAnalysis(typ domain.RequestType, size domain.RequestSize) []Stage {
	st := startConfirmed(typ, size)
	st[1].WantStatus = "planning"
	return st
}

// E19: a user who is not an approver is refused, and the refusal is in the audit log as denied.
func e19NotAnApprover() Scenario {
	sc := Scenario{ID: "E19", Group: GroupCrossCut, Type: string(domain.RequestTypeBug), Size: "M"}
	sc.Stages = []Stage{
		{
			Name:          "create_and_classify",
			Run:           func(t *testing.T, w *world) { w.request = w.k.create("not an approver", w.typ, w.size) },
			WantStatus:    "awaiting_type_confirmation",
			WantApprovals: []string{"request_type"},
		},
		{
			Name: "outsider_cannot_approve",
			Run: func(t *testing.T, w *world) {
				k := w.k
				list, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: w.request.GetId()})
				w.callOK(err, "ListApprovals")
				if len(list.GetApprovals()) == 0 {
					t.Fatal("no approval to refuse")
				}
				a := list.GetApprovals()[0]
				_, err = k.appr.Approve(k.as(k.outsider, "user"), &requestv1.ApproveRequest{Id: a.GetId(), ExpectedDigest: a.GetSubjectDigest()})
				if code(err) != codes.PermissionDenied || errText(err) == "" {
					t.Fatalf("outsider's Approve = %v, want PermissionDenied", err)
				}
				if !containsAll(errText(err), "REQUEST_APPROVAL_NOT_APPROVER") {
					t.Fatalf("want REQUEST_APPROVAL_NOT_APPROVER, got %v", err)
				}
				eventually(t, 10*time.Second, "the denied audit entry", func() (bool, string) {
					return hasAction(k.audit(), domain.ActionApprovalApprove, "denied"), "audit log has no approval.approve/denied"
				})
				// The approval is untouched: the Request still waits for its type to be confirmed.
				if got := k.get(w.request.GetId()); got.GetStatus() != "awaiting_type_confirmation" {
					t.Fatalf("status %s after a refused approval", got.GetStatus())
				}
			},
		},
	}
	return sc
}
