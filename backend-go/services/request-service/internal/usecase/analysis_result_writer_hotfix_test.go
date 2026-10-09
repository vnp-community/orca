package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// hotfixKit runs a hotfix diagnosis through the real runner and writer with an in-memory task-service.
func hotfixKit(t *testing.T) (*agentKit, *fixTaskFake) {
	t.Helper()
	k := newAgentKit(t, "```json\n"+analysisReply(t, "diagnosis_valid.json")+"\n```")
	tasks := newFixTaskFake()
	k.hotfix = NewEnsureHotfixFixTask(tasks, tasks, fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})}, domain.DefaultPlanLimits)
	return k, tasks
}

func TestHotfixDiagnosis_CreatesTheFixTaskAndOpensPreDeploy(t *testing.T) {
	k, tasks := hotfixKit(t)
	run, req := k.run(t, domain.RequestTypeHotfix)

	if run.Status != domain.RunStatusSucceeded || req.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("run = %s, request = %s", describe(run), req.Status)
	}
	if len(tasks.created) != 1 || len(tasks.tasks) != 1 {
		t.Fatalf("fix tasks created %d, held %d", len(tasks.created), len(tasks.tasks))
	}
	fix := tasks.created[0]
	if fix.Request.ID != req.ID || fix.Shape != domain.ShapeSingleTask {
		t.Fatalf("fix task input = %+v", fix)
	}
	if len(k.w.opened) != 1 {
		t.Fatalf("approvals opened = %d", len(k.w.opened))
	}
	o := k.w.opened[0]
	var fixID string
	for id := range tasks.tasks {
		fixID = id
	}
	if o.SubjectType != domain.SubjectPreDeploy || o.SubjectID != fixID {
		t.Fatalf("approval = %s on %q, want pre_deploy on the fix task %q", o.SubjectType, o.SubjectID, fixID)
	}
	// An approval records the status it is opened in, so it must come after the transition.
	if o.Request.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("approval opened while the request was %s", o.Request.Status)
	}
	if k.w.solutions[run.SolutionID].Status != domain.SolutionStatusApproved {
		t.Fatalf("diagnosis = %s, a hotfix diagnosis is auto-approved", k.w.solutions[run.SolutionID].Status)
	}
}

func TestHotfixDiagnosis_OtherTypesGetNoFixTask(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeBug, domain.RequestTypeSecurity, domain.RequestTypePerformance} {
		t.Run(string(typ), func(t *testing.T) {
			k, tasks := hotfixKit(t)
			run, _ := k.run(t, typ)
			if run.Status != domain.RunStatusSucceeded || len(tasks.created) != 0 {
				t.Fatalf("run = %s, fix tasks = %d", describe(run), len(tasks.created))
			}
			if len(k.w.opened) != 1 || k.w.opened[0].SubjectType != domain.SubjectSolution {
				t.Fatalf("approvals = %+v", k.w.opened)
			}
		})
	}
}

func TestHotfixDiagnosis_FixTaskFailureFailsTheRunAndKeepsAnalyzing(t *testing.T) {
	k, tasks := hotfixKit(t)
	tasks.createErr = domain.ErrHotfixFixTaskUnavailable(errors.New("dial tcp 10.1.2.3:9000: refused"))
	run, req := k.run(t, domain.RequestTypeHotfix)
	assertFailed(t, k.w, run, req, domain.RunErrHotfixFixTask)
	if msg := *run.ErrorMessage; msg == "" || strings.Contains(msg, "10.1.2.3") {
		t.Fatalf("run message %q must name the code and not the host", msg)
	}
}

func TestHotfixDiagnosis_ApprovalFailureRollsBackAndTheRetryReusesTheTask(t *testing.T) {
	k, tasks := hotfixKit(t)
	k.w.failOpen = domain.ErrApprovalNoEligibleApprover
	run, req := k.run(t, domain.RequestTypeHotfix)
	assertFailed(t, k.w, run, req, domain.RunErrHotfixApproval)
	if len(tasks.tasks) != 1 {
		t.Fatalf("the fix task must survive for the retry, held %d", len(tasks.tasks))
	}

	// The operator fixes the approver directory and runs the analysis again.
	k.w.failOpen = nil
	sp := &capturingSpawner{}
	if _, err := k.w.newGenerate(sp, fakeConns{conn: k.conn}).Execute(k.w.ctx(), GenerateSolutionInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	k.runner.execute(sp.runs[0])
	if got := k.w.requests[req.ID].Status; got != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s after the retry", got)
	}
	if len(tasks.created) != 1 || len(tasks.tasks) != 1 || len(k.w.opened) != 1 {
		t.Fatalf("retry created %d tasks (held %d), opened %d approvals", len(tasks.created), len(tasks.tasks), len(k.w.opened))
	}
}

// movingHotfix cancels the Request right after the fix task was made, as a user could while the model worked.
type movingHotfix struct {
	HotfixFixTasks
	w *solWorld
}

func (m movingHotfix) Ensure(ctx context.Context, req domain.Request, doc []byte) (HotfixFixTaskResult, error) {
	res, err := m.HotfixFixTasks.Ensure(ctx, req, doc)
	cur := m.w.requests[req.ID]
	cur.Status = domain.RequestStatusCancelled
	m.w.requests[req.ID] = cur
	return res, err
}

func TestHotfixDiagnosis_RequestMovedOnRetiresTheTaskItMade(t *testing.T) {
	k, tasks := hotfixKit(t)
	k.hotfix = movingHotfix{HotfixFixTasks: k.hotfix, w: k.w}
	run, _ := k.run(t, domain.RequestTypeHotfix)

	if run.Status != domain.RunStatusFailed || run.ErrorCode == nil || *run.ErrorCode != runErrRequestMoved {
		t.Fatalf("run = %s", describe(run))
	}
	for id, task := range tasks.tasks {
		if task.Status != domain.TaskStatusCancelled {
			t.Fatalf("task %s is %s, an orphan fix task must be cancelled", id, task.Status)
		}
	}
	if len(k.w.opened) != 0 {
		t.Fatalf("approvals = %+v", k.w.opened)
	}
}

func TestHotfixDiagnosis_WithoutTheGateBehavesAsBefore(t *testing.T) {
	k := newAgentKit(t, "```json\n"+analysisReply(t, "diagnosis_valid.json")+"\n```")
	run, req := k.run(t, domain.RequestTypeHotfix)
	if run.Status != domain.RunStatusSucceeded || req.Status != domain.RequestStatusAwaitingPlanApproval || len(k.w.opened) != 0 {
		t.Fatalf("run = %s, request = %s, approvals = %d", describe(run), req.Status, len(k.w.opened))
	}
}
