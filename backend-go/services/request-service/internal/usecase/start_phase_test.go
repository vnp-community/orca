package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func startPhase(r *exRig, req domain.Request, phaseID string) (StartPhaseResult, error) {
	return r.startPh.Execute(r.ctx(), StartPhaseInput{RequestID: req.ID, PhaseTaskID: phaseID})
}

func TestStartPhase_NotExecuting(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusAwaitingPlanApproval)
	ph := r.phase(req, r.plan(req), "p1")
	if _, err := startPhase(r, req, ph.ID); !errorHasCode(err, "REQUEST_NOT_EXECUTING") {
		t.Fatalf("got %v", err)
	}
}

func TestStartPhase_PhaseOfOtherRequest(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	r.phase(req, r.plan(req), "mine")
	other := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	foreign := r.phase(other, r.plan(other), "theirs")
	if _, err := startPhase(r, req, foreign.ID); !errorHasCode(err, "REQUEST_PHASE_NOT_IN_PLAN") {
		t.Fatalf("got %v", err)
	}
	if _, err := startPhase(r, req, "no-such-phase"); !errorHasCode(err, "REQUEST_PHASE_NOT_IN_PLAN") {
		t.Fatalf("unknown phase: %v", err)
	}
}

func TestStartPhase_NotApproved_ChangeRequest(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := startPhase(r, req, ph.ID); !errorHasCode(err, "REQUEST_PHASE_NOT_APPROVED") {
		t.Fatalf("no approval: %v", err)
	}
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPhase, SubjectID: ph.ID, Status: domain.ApprovalStatusPending})
	if _, err := startPhase(r, req, ph.ID); !errorHasCode(err, "REQUEST_PHASE_NOT_APPROVED") {
		t.Fatalf("pending approval: %v", err)
	}
	if r.tasks.count("execute:") != 0 || len(r.starts.rows) != 0 {
		t.Fatal("a refused start must change nothing")
	}
}

func TestStartPhase_NoPhaseGate_BugSizeL_OK(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	task := r.leaf(req, ph, "t", domain.TaskStatusOpen)
	res, err := startPhase(r, req, ph.ID)
	if err != nil || res.AlreadyStarted || len(res.Dispatched) != 1 || res.Dispatched[0] != task.ID {
		t.Fatalf("a bug has no phase gate: %+v %v", res, err)
	}
}

func TestStartPhase_PredecessorNotDone(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1 := r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusBlocked)
	r.dependsOn(p2, p1)
	r.approved(req, domain.SubjectPhase, p2.ID)
	if _, err := startPhase(r, req, p2.ID); !errorHasCode(err, "REQUEST_PHASE_PREDECESSOR_NOT_DONE") {
		t.Fatalf("got %v", err)
	}
	r.tasks.mu.Lock()
	r.tasks.setStatusLocked(t1.ID, domain.TaskStatusDone) // t1 done -> p1 derived done
	r.tasks.mu.Unlock()
	if _, err := startPhase(r, req, p2.ID); err != nil {
		t.Fatalf("predecessor done: %v", err)
	}
}

func TestStartPhase_TwiceReturnsAlreadyStarted_NoDoubleExecute(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)

	first, err := startPhase(r, req, ph.ID)
	if err != nil || first.AlreadyStarted {
		t.Fatalf("first start: %+v %v", first, err)
	}
	second, err := startPhase(r, req, ph.ID)
	if err != nil || !second.AlreadyStarted || len(second.Dispatched) != 0 {
		t.Fatalf("second start: %+v %v", second, err)
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("Execute must run once: %v", r.tasks.calls)
	}
	started := 0
	for _, s := range r.store.subjects() {
		if s == domain.SubjectPhaseStarted {
			started++
		}
	}
	if started != 1 {
		t.Fatalf("phase.started must be emitted once, got %d", started)
	}
	if r.starts.rows[0].StartedBy != "starter-1" {
		t.Fatalf("started_by must be the caller: %+v", r.starts.rows[0])
	}
}

func TestStartPhase_RunsWithTheStartersIdentity(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := startPhase(r, req, ph.ID); err != nil {
		t.Fatal(err)
	}
	if len(r.tasks.executedBy) != 1 || r.tasks.executedBy[0] != "starter-1" {
		t.Fatalf("got %v", r.tasks.executedBy)
	}
}

func TestStartPhase_Forbidden(t *testing.T) {
	r := newExRig(t)
	r.startPh.Authorizer = exAuthorizer{err: domain.ErrForbiddenToStart()}
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := startPhase(r, req, ph.ID); !errorHasCode(err, "REQUEST_START_FORBIDDEN") {
		t.Fatalf("got %v", err)
	}
	if len(r.starts.rows) != 0 {
		t.Fatal("a forbidden start must not claim the phase")
	}
}

func TestStartPhase_WholePlanWithoutPhases(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	res, err := startPhase(r, req, "")
	if err != nil || len(res.Dispatched) != 1 {
		t.Fatalf("got %+v %v", res, err)
	}
	phased := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	r.phase(phased, r.plan(phased), "p")
	if _, err := startPhase(r, phased, ""); !errorHasCode(err, "REQUEST_PHASE_REQUIRED") {
		t.Fatalf("a phased request needs a phase id: %v", err)
	}
}

func TestStartPhase_NeedsAUser(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	if _, err := r.startPh.Execute(lcCtx(), StartPhaseInput{RequestID: req.ID, PhaseTaskID: ph.ID}); err == nil {
		t.Fatal("no acting user, no start")
	}
}

func TestApprovalStartPhaseAuthorizer(t *testing.T) {
	req := domain.Request{ReporterID: "rep-1"}
	phaseApproval := &domain.Approval{}
	cases := []struct {
		name      string
		user      string
		role      string
		authorize error
		approval  *domain.Approval
		want      string
	}{
		{"reporter", "rep-1", "", errors.New("unused"), nil, ""},
		{"admin", "x", "admin", errors.New("unused"), nil, ""},
		{"phase approver", "ap-1", "", nil, phaseApproval, ""},
		{"approver who is also the requester", "ap-1", "", domain.ErrSelfApprovalForbidden, phaseApproval, ""},
		{"not an approver", "x", "", domain.ErrNotApprover, phaseApproval, "REQUEST_START_FORBIDDEN"},
		{"no approval record and not the owner", "x", "", nil, nil, "REQUEST_START_FORBIDDEN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := lcCtxWithUser(tc.user, tc.role)
			a := &ApprovalStartPhaseAuthorizer{Approvals: stubDecider{err: tc.authorize}}
			err := a.CanStart(ctx, req, tc.approval)
			if tc.want == "" && err != nil || tc.want != "" && !errorHasCode(err, tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
