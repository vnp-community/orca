package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type stubHub struct {
	yes   bool
	calls int
}

func (s *stubHub) Supports(context.Context, string) bool { s.calls++; return s.yes }

func pinned(e domain.EngineName) domain.Request {
	return domain.Request{ID: "r1", ProjectID: "p1", SolutionEngine: &e}
}

func TestOpenSpecContractRuns_OnlyTheTasksOfAnOpenSpecPlanGoOnTheContractPath(t *testing.T) {
	hub := &stubHub{yes: true}
	planner := OpenSpecContractRuns{Knowledge: hub, NewNonce: func() string { return "n0nce" }}
	task := domain.TaskView{ID: "t1", Description: "Serves: FR-1\nCode: modify `a.go`\nVerify: `go test`\nDone when: green."}

	run, ok := planner.Plan(context.Background(), pinned(domain.EngineOpenSpec), task)
	if !ok || run.ResultNonce != "n0nce" || !run.UseKnowledgeHub || !run.RequirePassingCheck {
		t.Fatalf("openspec plan: %+v ok=%v", run, ok)
	}
	for name, req := range map[string]domain.Request{"native": pinned(domain.EngineNative), "not pinned": {ID: "r2"}} {
		if _, ok := planner.Plan(context.Background(), req, task); ok {
			t.Errorf("a %s Request must keep the plain run", name)
		}
	}
	if hub.calls != 1 {
		t.Fatalf("the dev server was asked %d times; only an openspec task needs the answer", hub.calls)
	}
}

func TestOpenSpecContractRuns_AskForTheHubAndTheCheckOnlyWhereTheyApply(t *testing.T) {
	planner := OpenSpecContractRuns{Knowledge: &stubHub{yes: false}}
	run, ok := planner.Plan(context.Background(), pinned(domain.EngineOpenSpec), domain.TaskView{Description: "Serves: FR-1\nCode: modify `a.go`\nVerify:\nDone when: x"})
	if !ok || run.UseKnowledgeHub || run.RequirePassingCheck || len(run.ResultNonce) < 16 {
		t.Fatalf("no hub and an empty Verify line: %+v ok=%v", run, ok)
	}
	if _, ok := (OpenSpecContractRuns{}).Plan(context.Background(), pinned(domain.EngineOpenSpec), domain.TaskView{}); !ok {
		t.Fatal("a planner without a probe still plans the contract run, just without the hub")
	}
}

func TestCapabilityKnowledgeProbe_AnAgentThatDoesNotAdvertiseTheHubGetsNone(t *testing.T) {
	conns := fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}}
	cases := map[string]CapabilityKnowledgeProbe{
		"advertises":           {Conns: conns, Caps: fakeCaps{features: []string{domain.FeatureKnowledgeHub}}},
		"does not":             {Conns: conns, Caps: fakeCaps{features: []string{"agent.execPrompt"}}},
		"capabilities fail":    {Conns: conns, Caps: fakeCaps{err: context.DeadlineExceeded}},
		"no repo path":         {Conns: fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}, Caps: fakeCaps{features: []string{domain.FeatureKnowledgeHub}}},
		"no dev server":        {Conns: fakeConns{err: ErrNoDevServer}, Caps: fakeCaps{features: []string{domain.FeatureKnowledgeHub}}},
		"nothing wired":        {},
		"no capability reader": {Conns: conns},
	}
	want := map[string]bool{"advertises": true}
	for name, p := range cases {
		if got := p.Supports(context.Background(), "p1"); got != want[name] {
			t.Errorf("%s: got %v", name, got)
		}
	}
}

func TestAdvanceExecution_AnOpenSpecTaskIsDispatchedOnTheContractPathWhenTheClientCanAndOtherwiseOnThePlainOne(t *testing.T) {
	openspec := domain.EngineOpenSpec
	setup := func(engine *domain.EngineName, plan ContractRunPlanner, contract bool) (*exRig, *contractTasks, domain.Request) {
		r := newExRig(t)
		req := r.store.seed(func(q *domain.Request) {
			q.Type, q.Size, q.Status, q.SolutionEngine = domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting, engine
		})
		p := r.plan(req)
		task := r.leaf(req, p, "do it", domain.TaskStatusOpen)
		task.Description = "Serves: FR-1\nCode: modify `a.go`\nVerify: `go test`\nDone when: green."
		r.tasks.tasks[task.ID] = task
		ct := &contractTasks{exTasks: r.tasks}
		r.advance.Contract = plan
		if contract {
			r.advance.Tasks = ct
		}
		return r, ct, req
	}
	planner := OpenSpecContractRuns{NewNonce: func() string { return "n0nce-n0nce-n0nce-n0nce" }}

	r, ct, req := setup(&openspec, planner, true)
	advance(t, r, req, "")
	if len(ct.runs) != 1 || ct.runs[0].ResultNonce != "n0nce-n0nce-n0nce-n0nce" || !ct.runs[0].RequirePassingCheck {
		t.Fatalf("contract runs = %+v", ct.runs)
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("one dispatch only, calls: %v", r.tasks.calls)
	}

	r, ct, req = setup(&openspec, planner, false) // a client that cannot do contract runs
	advance(t, r, req, "")
	if r.tasks.count("execute:") != 1 || len(ct.runs) != 0 {
		t.Fatalf("plain=%d contract=%d", r.tasks.count("execute:"), len(ct.runs))
	}

	native := domain.EngineNative
	r, ct, req = setup(&native, planner, true)
	advance(t, r, req, "")
	if r.tasks.count("execute:") != 1 || len(ct.runs) != 0 {
		t.Fatalf("a native Request must stay plain: plain=%d contract=%d", r.tasks.count("execute:"), len(ct.runs))
	}

	r, ct, req = setup(&openspec, nil, true) // no planner wired
	advance(t, r, req, "")
	if r.tasks.count("execute:") != 1 || len(ct.runs) != 0 {
		t.Fatalf("no planner, plain=%d contract=%d", r.tasks.count("execute:"), len(ct.runs))
	}
}

// contractTasks is exTasks that can also take a contract dispatch; it records the run and then behaves like Execute.
type contractTasks struct {
	*exTasks
	runs []ContractRun
}

func (c *contractTasks) ExecuteContract(ctx context.Context, taskID, ref string, run ContractRun) error {
	c.runs = append(c.runs, run)
	return c.exTasks.Execute(ctx, taskID, ref)
}
