//go:build e2e

package e2e

import (
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

// E13: a Request returned to the backlog can be reopened and is classified again. The CR's variant starts from a
// rejected Plan; the analysis-stage return runs now, the plan rejection waits for the Plan RPCs.
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
			Name:  "plan_rejection_returns_to_backlog",
			Needs: "CR-REQ-012 (GeneratePlan, plan approval)",
			Run:   func(t *testing.T, w *world) { probeRPC(t, w, "GeneratePlan") },
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

func e15TaskFailureReturnsToBacklog() Scenario {
	sc := Scenario{ID: "E15", Group: GroupCrossCut, Type: string(domain.RequestTypeTask), Size: "S", Stages: startConfirmedNoAnalysis(domain.RequestTypeTask, domain.RequestSizeS)}
	sc.Stages = append(sc.Stages, Stage{
		Name:  "failed_task_returns_request_to_backlog",
		Needs: "CR-REQ-013 (ReportTaskOutcome with a failed outcome)",
		Run:   func(t *testing.T, w *world) { probeRPC(t, w, "ReportTaskOutcome") },
	})
	return sc
}

func e16OutcomeCallbackIdempotent() Scenario {
	sc := Scenario{ID: "E16", Group: GroupCrossCut, Type: string(domain.RequestTypeTask), Size: "S", Stages: startConfirmedNoAnalysis(domain.RequestTypeTask, domain.RequestSizeS)}
	sc.Stages = append(sc.Stages, Stage{
		Name:  "repeated_and_reordered_callbacks_change_nothing",
		Needs: "CR-REQ-013 (ReportTaskOutcome)",
		Run:   func(t *testing.T, w *world) { probeRPC(t, w, "ReportTaskOutcome") },
	})
	return sc
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
