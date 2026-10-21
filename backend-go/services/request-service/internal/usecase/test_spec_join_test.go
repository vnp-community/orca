package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func (l *tsLane) join(gate domain.TestSpecGate) *TestSpecJoin {
	return &TestSpecJoin{
		Runs: l.runs, Gate: gate, Timeout: 30 * time.Minute, Now: func() time.Time { return l.now },
		Settings: TestSpecSettingsFunc(func(context.Context, string) bool { return l.enabled }),
	}
}

func TestTheLaneStartsWhenPlanGenerationBegins(t *testing.T) {
	for _, st := range []domain.RequestStatus{domain.RequestStatusPlanning, domain.RequestStatusAwaitingPlanApproval} {
		l := newLane()
		req := l.reqs.reqs[tsRequest]
		req.Status = st
		l.reqs.reqs[tsRequest] = req
		l.solutions.list[0].ChosenOption = ptr(0)
		started, err := l.start().Execute(tsCtx(), tsRequest)
		if err != nil || !started || l.runs.status(tsRequest).State != domain.TestSpecRunning {
			t.Fatalf("%s: started=%v err=%v row=%+v", st, started, err, l.runs.status(tsRequest))
		}
	}
	// Before the Solution is approved there is nothing to read and no Plan to run beside.
	l := newLane()
	req := l.reqs.reqs[tsRequest]
	req.Status = domain.RequestStatusAnalyzing
	l.reqs.reqs[tsRequest] = req
	if started, _ := l.start().Execute(tsCtx(), tsRequest); started || l.agent.count("start") != 0 {
		t.Fatal("an analyzing Request must not start the lane")
	}
}

func TestTheJoinReadsTheLaneOfTheRequest(t *testing.T) {
	l := newLane()
	req := l.reqs.reqs[tsRequest]
	j := l.join(domain.TestSpecGateWait)
	if d, err := j.Decide(tsCtx(), req); err != nil || d.Verdict != domain.JoinGo {
		t.Fatalf("a Request without a lane is never held: %+v %v", d, err)
	}
	l.runs.rows = append(l.runs.rows, domain.TestSpecRun{ID: "r", RequestID: tsRequest, State: domain.TestSpecRunning, StartedAt: l.now})
	if d, _ := j.Decide(tsCtx(), req); d.Verdict != domain.JoinWait {
		t.Fatalf("a generating lane holds execution: %+v", d)
	}
	if err := j.Enforce(tsCtx(), req); err == nil {
		t.Fatal("Enforce refuses while the lane generates")
	}
	l.now = l.now.Add(31 * time.Minute)
	if d, _ := j.Decide(tsCtx(), req); d.Verdict != domain.JoinGo || !d.Warn {
		t.Fatalf("the wait runs out and execution goes on with a warning: %+v", d)
	}
	l.enabled = false
	l.runs.rows[0].StartedAt = l.now
	if d, _ := l.join(domain.TestSpecGateRequire).Decide(tsCtx(), req); d.Verdict != domain.JoinGo {
		t.Fatalf("a project without test specs is never held: %+v", d)
	}
}

func TestExecutionWaitsForTheLaneThenStarts(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	l := newLane()
	l.runs.rows = append(l.runs.rows, domain.TestSpecRun{ID: "r", RequestID: req.ID, State: domain.TestSpecRunning, StartedAt: l.now})
	r.advance.Join = l.join(domain.TestSpecGateWait)

	started, err := r.startExec.Execute(lcCtx(), req.ID)
	if err != nil || r.tasks.get(task.ID).Status != domain.TaskStatusOpen {
		t.Fatalf("plan ready, test specs generating: the first dispatch waits without an error: started=%v err=%v", started, err)
	}
	l.runs.rows[0].State = domain.TestSpecCompleted
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || r.tasks.get(task.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("both tracks finished: execution starts: err=%v calls=%v", err, r.tasks.calls)
	}
}

func TestTheJoinOnlyGuardsTheFirstDispatch(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	r.leaf(req, plan, "a", domain.TaskStatusDone)
	second := r.leaf(req, plan, "b", domain.TaskStatusOpen)
	l := newLane()
	l.runs.rows = append(l.runs.rows, domain.TestSpecRun{ID: "r", RequestID: req.ID, State: domain.TestSpecRunning, StartedAt: l.now})
	r.advance.Join = l.join(domain.TestSpecGateRequire)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || r.tasks.get(second.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("a Re-run of the test specs must not stop a Request that is already executing: err=%v calls=%v", err, r.tasks.calls)
	}
}

func TestExecutionIsToldWhenTheLaneFinishes(t *testing.T) {
	l := newLane()
	l.agent.cfg.Enabled = false // the dev server says test specs are off: the lane ends at once as skipped
	var told []string
	st := l.start()
	st.OnFinished = func(_ context.Context, id string) { told = append(told, id) }
	if started, err := st.Execute(tsCtx(), tsRequest); err != nil || !started {
		t.Fatalf("started=%v err=%v", started, err)
	}
	if len(told) != 1 || told[0] != tsRequest || l.runs.status(tsRequest).State != domain.TestSpecSkipped {
		t.Fatalf("OnFinished must fire once for a lane that ended: told=%v row=%+v", told, l.runs.status(tsRequest))
	}
}
