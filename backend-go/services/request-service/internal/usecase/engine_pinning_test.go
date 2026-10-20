package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type fakeEngineSettings struct {
	byProject map[string]domain.ProjectEngineSettings
	err       error
}

func (f fakeEngineSettings) Get(_ context.Context, projectID string) (domain.ProjectEngineSettings, bool, error) {
	s, ok := f.byProject[projectID]
	return s, ok, f.err
}

func (fakeEngineSettings) Upsert(context.Context, domain.ProjectEngineSettings, int64) (domain.ProjectEngineSettings, error) {
	return domain.ProjectEngineSettings{}, errors.New("unused")
}

func openspecSettings(projectID string) fakeEngineSettings {
	return fakeEngineSettings{byProject: map[string]domain.ProjectEngineSettings{projectID: {ProjectID: projectID, Engine: domain.EngineOpenSpec}}}
}

func TestPinSolutionEngine_DecidesFromSettingsAndTypeProfile(t *testing.T) {
	cases := []struct {
		name string
		typ  domain.RequestType
		set  bool
		want domain.EngineName
	}{
		{"no settings row means native", domain.RequestTypeChangeRequest, false, domain.EngineNative},
		{"openspec project, full profile type", domain.RequestTypeChangeRequest, true, domain.EngineOpenSpec},
		{"openspec project, light profile type", domain.RequestTypeBug, true, domain.EngineOpenSpec},
		{"openspec project, type without a profile stays native", domain.RequestTypeTask, true, domain.EngineNative},
		{"openspec project, question stays native", domain.RequestTypeQuestion, true, domain.EngineNative},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newSolWorld()
			req := w.seedRequest(tc.typ, domain.RequestStatusAnalyzing)
			settings := fakeEngineSettings{}
			if tc.set {
				settings = openspecSettings(req.ProjectID)
			}
			got, err := NewPinSolutionEngine(w, settings).Execute(w.ctx(), req.ID)
			if err != nil || got != tc.want {
				t.Fatalf("engine = %q, %v; want %q", got, err, tc.want)
			}
			cur, _ := w.Get(w.ctx(), req.ID)
			if cur.SolutionEngine == nil || *cur.SolutionEngine != tc.want {
				t.Fatalf("not pinned on the request: %v", cur.SolutionEngine)
			}
			if cur.Version != req.Version {
				t.Fatalf("pinning must not bump the version: %d -> %d", req.Version, cur.Version)
			}
		})
	}
}

func TestPinSolutionEngine_IsIdempotentWhenTheSettingChangesLater(t *testing.T) {
	w := newSolWorld()
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	first, err := NewPinSolutionEngine(w, fakeEngineSettings{}).Execute(w.ctx(), req.ID)
	if err != nil || first != domain.EngineNative {
		t.Fatalf("first = %q, %v", first, err)
	}
	// The project is switched to openspec after the first generation: the Request keeps its engine.
	again, err := NewPinSolutionEngine(w, openspecSettings(req.ProjectID)).Execute(w.ctx(), req.ID)
	if err != nil || again != domain.EngineNative {
		t.Fatalf("a pinned Request must keep its engine, got %q, %v", again, err)
	}
}

func TestPinSolutionEngine_Errors(t *testing.T) {
	w := newSolWorld()
	if _, err := NewPinSolutionEngine(w, fakeEngineSettings{}).Execute(w.ctx(), "missing"); err == nil {
		t.Fatal("an unknown request must fail")
	}
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	boom := errors.New("settings store down")
	if _, err := NewPinSolutionEngine(w, fakeEngineSettings{err: boom}).Execute(w.ctx(), req.ID); !errors.Is(err, boom) {
		t.Fatalf("a settings failure must surface, got %v", err)
	}
	cur, _ := w.Get(w.ctx(), req.ID)
	if cur.SolutionEngine != nil {
		t.Fatal("nothing may be pinned when the decision failed")
	}
}

type stubPinner struct {
	engine domain.EngineName
	err    error
	calls  int
}

func (p *stubPinner) Execute(context.Context, string) (domain.EngineName, error) {
	p.calls++
	return p.engine, p.err
}

func planKitWithPin(pin EnginePinner, conns AnalysisConnectionResolver) *planKit {
	k := newPlanKit(conns)
	k.generate = NewGeneratePlan(GeneratePlanDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Runs: fakeRuns{k.w}, Conns: conns, Auth: NewPlanActorAuthorizer(allowAll{}, nil, false), Approvals: fakeCanceller{k.w},
		Transition: k.w.transitioner(), Tx: k.w, Spawner: k.spawner, LeaseOwner: testOwner, Pin: pin,
	})
	return k
}

func TestGeneratePlan_NativeEngineKeepsTheCompletePath(t *testing.T) {
	for _, pin := range []EnginePinner{nil, &stubPinner{engine: domain.EngineNative}} {
		k := planKitWithPin(pin, fakeConns{})
		req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
		k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
		run := k.startedRun(req.ID)
		if run.Mode != domain.AnalysisModeComplete {
			t.Fatalf("native must keep ai.complete, got %s", run.Mode)
		}
	}
}

func TestGeneratePlan_OpenSpecEngineStartsAnAgentProposalRun(t *testing.T) {
	pin := &stubPinner{engine: domain.EngineOpenSpec}
	k := planKitWithPin(pin, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/repo"}})
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	run := k.startedRun(req.ID)
	if run.Kind != domain.RunKindPlan || run.Mode != domain.AnalysisModeAgentProposal || run.SolutionID != "" {
		t.Fatalf("run = %s", describe(run))
	}
	if pin.calls != 1 {
		t.Fatalf("the engine is decided once per start, got %d", pin.calls)
	}
}

func TestGeneratePlan_OpenSpecNeedsARepoPathAndFailsBeforeAnyWrite(t *testing.T) {
	k := planKitWithPin(&stubPinner{engine: domain.EngineOpenSpec}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	_, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	if err == nil || errCode(err) != "REQUEST_ANALYSIS_NO_REPO_PATH" {
		t.Fatalf("want REQUEST_ANALYSIS_NO_REPO_PATH, got %v", err)
	}
	if len(k.w.runs) != 0 || len(k.spawner.runs) != 0 {
		t.Fatal("no run row and no worker without a repo")
	}
}

func TestGeneratePlan_PinFailureStopsTheStart(t *testing.T) {
	boom := errors.New("pin failed")
	k := planKitWithPin(&stubPinner{err: boom}, fakeConns{})
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID}); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if len(k.w.runs) != 0 {
		t.Fatal("a failed pin must not leave a run")
	}
}

func TestGenerateSolution_PinsTheEngineAtTheFirstGeneration(t *testing.T) {
	w := newSolWorld()
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	pin := &stubPinner{engine: domain.EngineNative}
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{}).WithEnginePin(pin)
	if _, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	if pin.calls != 1 {
		t.Fatalf("pin calls = %d", pin.calls)
	}
}

func TestAnalysisRunner_AgentProposalWithoutWorkerFailsClosed(t *testing.T) {
	r := &AnalysisRunner{}
	run := domain.AnalysisRun{Kind: domain.RunKindPlan, Mode: domain.AnalysisModeAgentProposal}
	err := r.worker(run).Execute(context.Background(), run)
	var f *AnalysisFailure
	if !errors.As(err, &f) || f.Code != RunErrEngineUnavailable {
		t.Fatalf("got %v", err)
	}
}

func TestAnalysisRunner_AgentProposalUsesTheProposalWorkerWhenWired(t *testing.T) {
	proposal := &stubWorker{}
	plan := &stubWorker{}
	r := (&AnalysisRunner{}).WithPlanWorker(plan).WithProposalWorker(proposal)
	if r.worker(domain.AnalysisRun{Kind: domain.RunKindPlan, Mode: domain.AnalysisModeAgentProposal}) != proposal {
		t.Fatal("agent_proposal plan runs go to the proposal worker")
	}
	if r.worker(domain.AnalysisRun{Kind: domain.RunKindPlan, Mode: domain.AnalysisModeComplete}) != plan {
		t.Fatal("complete plan runs keep the plan worker")
	}
}

type stubWorker struct{}

func (*stubWorker) Execute(context.Context, domain.AnalysisRun) error { return nil }

func TestAnalysisMode_AgentSharesOneCapAndTheEngineFollowsTheMode(t *testing.T) {
	for mode, agent := range map[domain.AnalysisMode]bool{
		domain.AnalysisModeComplete: false, domain.AnalysisModeAgentReadonly: true, domain.AnalysisModeAgentProposal: true,
	} {
		if mode.IsAgent() != agent {
			t.Fatalf("%s IsAgent = %v", mode, mode.IsAgent())
		}
	}
	if domain.AnalysisModeAgentProposal.EngineFor() != domain.EngineOpenSpec || domain.AnalysisModeAgentReadonly.EngineFor() != domain.EngineNative {
		t.Fatal("only agent_proposal belongs to the openspec engine")
	}
}

func TestGeneratePlan_OpenSpecFailsFastWhenTheServerHasNoSuchEngine(t *testing.T) {
	k := planKitWithPin(&stubPinner{engine: domain.EngineOpenSpec}, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/repo"}})
	k.generate.registry = NewDefaultEngineRegistry()
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	_, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	if errCode(err) != "REQUEST_ENGINE_UNAVAILABLE" || len(k.w.runs) != 0 {
		t.Fatalf("want REQUEST_ENGINE_UNAVAILABLE and no run, got %v runs=%d", err, len(k.w.runs))
	}
	k.generate.registry = registryWith(&scriptedEngine{})
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID}); err != nil {
		t.Fatalf("with the engine registered the run starts: %v", err)
	}
}
