//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
)

// The driver does what the people and agents of a flow do, one stage at a time, through the public RPCs only.
// Approvals are decided by the tenant admin (who is never the reporter, so the gates that forbid self approval open too);
// task runs are ended by the task-service fake, whose events come back through ReportTaskOutcome.

// retry calls an RPC again while the per-tenant rate limiter refuses it: the limits are part of the service and stay on.
func retry[T any](k *kit, what string, call func() (T, error)) T {
	k.t.Helper()
	for attempt := 0; ; attempt++ {
		v, err := call()
		if err == nil {
			return v
		}
		if code(err) != codes.ResourceExhausted || attempt >= 80 {
			k.t.Fatalf("%s: %v", what, err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (w *world) id() string { return w.request.GetId() }

func (w *world) requestType() domain.RequestType { return domain.RequestType(w.typ) }

func (w *world) flow() domain.FlowDefinition {
	f, err := domain.FlowFor(w.requestType())
	if err != nil {
		w.k.t.Fatal(err)
	}
	return f
}

func (w *world) phased() bool { return w.flow().PhasesFor(domain.RequestSize(w.size)) }

// pendingApproval waits for the single pending approval of the given subjects on this Request.
func (w *world) pendingApproval(what string, subjects ...string) *requestv1.Approval {
	k := w.k
	var found *requestv1.Approval
	eventually(k.t, 20*time.Second, "a pending "+what+" approval", func() (bool, string) {
		resp, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: w.id(), Status: requestv1.ApprovalStatus_APPROVAL_STATUS_PENDING})
		if err != nil {
			return false, err.Error()
		}
		var have []string
		for _, a := range resp.GetApprovals() {
			have = append(have, subjectName(a.GetSubjectType()))
			if contains(subjects, subjectName(a.GetSubjectType())) {
				found = a
				return true, ""
			}
		}
		return false, fmt.Sprintf("pending approvals: %v", have)
	})
	return found
}

// approve decides an approval as the tenant admin with the digest the approver saw.
func (w *world) approve(a *requestv1.Approval, digest string) *requestv1.ApproveResponse {
	return w.approveWith(w.k.asAdmin(), a, digest)
}

func (w *world) approveWith(ctx context.Context, a *requestv1.Approval, digest string) *requestv1.ApproveResponse {
	k := w.k
	if digest == "" {
		digest = a.GetSubjectDigest()
	}
	return retry(k, "Approve "+subjectName(a.GetSubjectType()), func() (*requestv1.ApproveResponse, error) {
		return k.appr.Approve(ctx, &requestv1.ApproveRequest{Id: a.GetId(), Comment: "approved by the e2e admin", ExpectedDigest: digest})
	})
}

// runDone waits until a Solution or Plan run has left "running" and fails the test with the run's own error.
func (w *world) solveAndWait() *requestv1.Solution {
	k := w.k
	gen := retry(k, "GenerateSolution", func() (*requestv1.GenerateSolutionResponse, error) {
		return k.req.GenerateSolution(k.asReporter(), &requestv1.GenerateSolutionRequest{RequestId: w.id(), AnalysisMode: requestv1.AnalysisMode_ANALYSIS_MODE_COMPLETE})
	})
	var sol *requestv1.Solution
	eventually(k.t, 30*time.Second, "the analysis run "+gen.GetRunId(), func() (bool, string) {
		resp, err := k.req.ListSolutions(k.asReporter(), &requestv1.ListSolutionsRequest{RequestId: w.id()})
		if err != nil {
			return false, err.Error()
		}
		for _, r := range resp.GetRuns() {
			if r.GetId() != gen.GetRunId() {
				continue
			}
			switch r.GetStatus() {
			case "failed":
				k.t.Fatalf("analysis run failed: %s: %s", r.GetErrorCode(), r.GetErrorMessage())
			case "succeeded":
				for _, s := range resp.GetSolutions() {
					if s.GetId() == gen.GetSolutionId() {
						sol = s
					}
				}
				return sol != nil, "the run succeeded but its Solution is not listed"
			}
			return false, "run " + r.GetStatus()
		}
		return false, "run not listed yet"
	})
	return sol
}

// analyze is the analyzing stage: the stub model answers, the run commits, the gate (if any) opens.
func (w *world) analyze() { w.solveAndWait() }

// approveAnalysis chooses the recommended option when the Solution has options, then approves the analysis gate.
func (w *world) approveAnalysis() {
	k := w.k
	gate := string(w.flow().AnalysisGate)
	sol := w.latestSolution()
	digest := ""
	if sol.GetKind() == requestv1.SolutionKind_SOLUTION_KIND_SOLUTION {
		chosen := retry(k, "ChooseSolutionOption", func() (*requestv1.ChooseSolutionOptionResponse, error) {
			// An admin chooses: for a large request the reporter may not choose for their own request.
			return k.req.ChooseSolutionOption(k.asAdmin(), &requestv1.ChooseSolutionOptionRequest{RequestId: w.id(), SolutionId: sol.GetId(), OptionId: "opt-1", Comment: "the recommended option"})
		})
		digest = chosen.GetApprovalDigest()
	}
	a := w.pendingApproval(gate, gate)
	w.approve(a, digest)
}

func (w *world) latestSolution() *requestv1.Solution {
	k := w.k
	resp := retry(k, "ListSolutions", func() (*requestv1.ListSolutionsResponse, error) {
		return k.req.ListSolutions(k.asReporter(), &requestv1.ListSolutionsRequest{RequestId: w.id(), Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED})
	})
	if len(resp.GetSolutions()) == 0 {
		k.t.Fatalf("no proposed Solution to approve for %s", w.id())
	}
	return resp.GetSolutions()[0]
}

// proposePlan asks for a Plan and waits for the proposal; commitPlan stores it.
func (w *world) proposePlan() *requestv1.GetPlanProposalResponse {
	k := w.k
	if w.requestType() == domain.RequestTypePerformance {
		w.record(domain.CheckPerfBaseline, `{"metrics":[{"name":"p95_latency_ms","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":10}],"method":"wrk -t4"}`, "baseline measured")
	}
	gen := retry(k, "GeneratePlan", func() (*requestv1.GeneratePlanResponse, error) {
		return k.req.GeneratePlan(k.asReporter(), &requestv1.GeneratePlanRequest{RequestId: w.id()})
	})
	var out *requestv1.GetPlanProposalResponse
	eventually(k.t, 30*time.Second, "the plan run "+gen.GetRunId(), func() (bool, string) {
		resp, err := k.req.GetPlanProposal(k.asReporter(), &requestv1.GetPlanProposalRequest{RequestId: w.id(), RunId: gen.GetRunId()})
		if err != nil {
			return false, err.Error()
		}
		switch resp.GetStatus() {
		case "failed":
			k.t.Fatalf("plan run failed: %s: %s", resp.GetErrorCode(), resp.GetErrorMessage())
		case "succeeded":
			out = resp
			return true, ""
		}
		return false, "run " + resp.GetStatus()
	})
	return out
}

func (w *world) commitPlan(p *requestv1.GetPlanProposalResponse) *requestv1.CommitPlanResponse {
	k := w.k
	return retry(k, "CommitPlan", func() (*requestv1.CommitPlanResponse, error) {
		return k.req.CommitPlan(k.asReporter(), &requestv1.CommitPlanRequest{RequestId: w.id(), Proposal: p.GetProposal(), RawAiResponse: p.GetRawAiResponse()})
	})
}

func (w *world) plan() { w.commitPlan(w.proposePlan()) }

// approvePlan decides the gate that holds awaiting_plan_approval: plan, task_list or pre_deploy.
func (w *world) approvePlan() {
	gate := string(w.flow().StartGate)
	w.approve(w.pendingApproval(gate, gate), "")
}

// record stores a check as the reporter (a person), which is what the policies that need a human word accept.
func (w *world) record(kind domain.CheckKind, metrics, summary string) {
	k := w.k
	retry(k, "RecordRequestCheck "+string(kind), func() (*requestv1.RecordRequestCheckResponse, error) {
		return k.req.RecordRequestCheck(k.asReporter(), &requestv1.RecordRequestCheckRequest{
			RequestId: w.id(), Kind: string(kind), Status: "passed", MetricsJson: metrics, Summary: summary})
	})
}

// recordCompletionChecks gives the type policy what it wants to see before it lets the Request complete.
func (w *world) recordCompletionChecks() {
	switch w.requestType() {
	case domain.RequestTypePerformance:
		w.record(domain.CheckPerfAfter, `{"metrics":[{"name":"p95_latency_ms","value":100}]}`, "measured again")
	case domain.RequestTypeRefactor:
		w.record(domain.CheckTestsBefore, `{"total":40,"passed":40,"failed":0,"command":"go test ./..."}`, "before")
		w.record(domain.CheckTestsAfter, `{"total":40,"passed":40,"failed":0,"command":"go test ./...","tests_modified":false}`, "after")
	case domain.RequestTypeSecurity:
		w.record(domain.CheckSecurityRecheck, `{}`, "re-checked the fixed path; no regression found")
	case domain.RequestTypeOpsRequest:
		w.record(domain.CheckOpsResult, `{}`, "migration applied, service healthy")
	}
}

// requireNoGrantBeyondExecute: the people who drive execution without having committed the Plan get 'user' grants,
// never owner or admin, which would let them manage the tree.
func (w *world) requireNoGrantBeyondExecute() {
	for _, t := range theStack.tasks.TasksOfRequest(w.id()) {
		for _, g := range theStack.tasks.Grants(t.GetId()) {
			if !strings.HasSuffix(g, ":GRANT_LEVEL_USER") {
				w.k.t.Errorf("task %s has grant %q: execution must only ever grant the user level", t.GetId(), g)
			}
		}
	}
}

const executionBudget = 90 * time.Second

// execute runs the Request from executing to completed: it starts each Phase (after its approval where the flow gates
// Phases), approves task-level gates, and ends every dispatched task run successfully.
func (w *world) execute() {
	k := w.k
	w.recordCompletionChecks()
	phased, tasks := w.phased(), theStack.tasks
	finished := map[string]bool{}
	started := map[string]bool{}
	startErr, notBefore := map[string]string{}, map[string]time.Time{}
	deadline := time.Now().Add(executionBudget)
	for {
		req := k.get(w.id())
		if req.GetStatus() == string(domain.RequestStatusCompleted) {
			w.request = req
			w.requireNoGrantBeyondExecute()
			return
		}
		if req.GetStatus() != string(domain.RequestStatusExecuting) {
			k.t.Fatalf("the Request left executing for %s (%s: %s); task events failed: %v", req.GetStatus(), req.GetReturnedFromStage(), req.GetReturnReason(), tasks.deliveryErrors())
		}
		if time.Now().After(deadline) {
			k.t.Fatalf("not completed after %s; tasks: %s; phase start errors: %v; event errors: %v\nservice log tail:\n%s",
				executionBudget, describeTasks(tasks.TasksOfRequest(w.id())), startErr, tasks.deliveryErrors(), theStack.logTail(30))
		}
		if phased {
			w.startReadyPhases(tasks.TasksOfRequest(w.id()), started, startErr, notBefore)
		}
		w.approveTaskGates()
		for _, t := range tasks.TasksOfRequest(w.id()) {
			if t.GetStatus() == "in_progress" && !finished[t.GetId()] && t.GetTaskType() != "plan" && t.GetTaskType() != "phase" {
				finished[t.GetId()] = true
				tasks.RunSucceeded(t.GetId())
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// phaseRetryPause keeps a refused Phase start from being retried every round: each call spends a rate token.
const phaseRetryPause = 2 * time.Second

// startReadyPhases starts the Phases in order. Where the flow gates Phases, an admin approves the Phase first. A start the
// service refuses is retried after a pause, and its last error is kept for the failure message.
func (w *world) startReadyPhases(all []*taskv1.Task, started map[string]bool, lastErr map[string]string, notBefore map[string]time.Time) {
	k := w.k
	gated := w.flow().HasExecutionGate(domain.GatePhase)
	for _, t := range all {
		if t.GetTaskType() != "phase" {
			continue
		}
		if started[t.GetId()] || t.GetStatus() == "cancelled" {
			continue
		}
		if t.GetStatus() == "done" {
			continue
		}
		if time.Now().Before(notBefore[t.GetId()]) {
			return
		}
		if gated && !w.phaseApproved(t.GetId()) {
			lastErr[t.GetId()] = "no approved Phase approval yet"
			notBefore[t.GetId()] = time.Now().Add(300 * time.Millisecond)
			return
		}
		_, err := k.req.StartPhase(k.asReporter(), &requestv1.StartPhaseRequest{RequestId: w.id(), PhaseTaskId: t.GetId()})
		switch {
		case err == nil:
			started[t.GetId()] = true
			delete(lastErr, t.GetId())
		case code(err) == codes.FailedPrecondition && strings.Contains(err.Error(), "PREDECESSOR"):
			return // an earlier Phase is still running
		default:
			lastErr[t.GetId()] = err.Error()
			notBefore[t.GetId()] = time.Now().Add(phaseRetryPause)
			return // Phases start in order: the next one waits for this one
		}
	}
}

// phaseApproved decides the pending approval of a Phase (as an admin) and reports whether the Phase is approved.
func (w *world) phaseApproved(phaseID string) bool {
	k := w.k
	resp, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: w.id()})
	if err != nil {
		return false
	}
	for _, a := range resp.GetApprovals() {
		if subjectName(a.GetSubjectType()) != "phase" || a.GetSubjectId() != phaseID {
			continue
		}
		switch a.GetStatus() {
		case requestv1.ApprovalStatus_APPROVAL_STATUS_APPROVED:
			return true
		case requestv1.ApprovalStatus_APPROVAL_STATUS_PENDING:
			w.approve(a, "")
			return true
		}
	}
	return false
}

// approveTaskGates decides pre_deploy approvals that belong to a single task (the runbook steps of an ops_request).
func (w *world) approveTaskGates() {
	k := w.k
	if !w.flow().HasExecutionGate(domain.GatePreDeploy) {
		return
	}
	resp, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: w.id(), Status: requestv1.ApprovalStatus_APPROVAL_STATUS_PENDING})
	if err != nil {
		return
	}
	for _, a := range resp.GetApprovals() {
		if subjectName(a.GetSubjectType()) == "pre_deploy" {
			// The first admin approved the Plan, so the gate was opened in their name and they may not decide it.
			w.approveWith(k.asAdmin2(), a, "")
		}
	}
}

func describeTasks(ts []*taskv1.Task) string {
	var parts []string
	for _, t := range ts {
		parts = append(parts, fmt.Sprintf("%s %q %s", t.GetTaskType(), t.GetTitle(), t.GetStatus()))
	}
	return strings.Join(parts, "; ")
}
