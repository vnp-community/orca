package usecase

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestModeFor_SolutionsStayOnCompleteUnlessTheProjectOptedIn(t *testing.T) {
	const (
		sol  = domain.SolutionKindSolution
		diag = domain.SolutionKindDiagnosis
	)
	cases := []struct {
		name     string
		kind     domain.SolutionKind
		override domain.AnalysisMode
		optedIn  bool
		want     domain.AnalysisMode
		wantErr  bool
	}{
		{"solution, no opt-in", sol, "", false, domain.AnalysisModeComplete, false},
		{"solution, opt-in", sol, "", true, domain.AnalysisModeAgentReadonly, false},
		{"solution, caller asks for the agent without opt-in", sol, domain.AnalysisModeAgentReadonly, false, "", true},
		{"solution, caller asks for the agent with opt-in", sol, domain.AnalysisModeAgentReadonly, true, domain.AnalysisModeAgentReadonly, false},
		{"solution, caller asks for complete with opt-in", sol, domain.AnalysisModeComplete, true, domain.AnalysisModeComplete, false},
		{"diagnosis ignores the opt-in", diag, "", false, domain.AnalysisModeAgentReadonly, false},
		{"diagnosis may still ask for complete", diag, domain.AnalysisModeComplete, true, domain.AnalysisModeComplete, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := modeFor(tc.kind, tc.override, tc.optedIn)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v; want %q (err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

const hubFeature = domain.FeatureKnowledgeHub

// solutionAgentKit starts a change_request Solution for a project that may have opted in.
type solutionAgentKit struct {
	*agentKit
	settings fakeEngineSettings
	features []string
	conn     AnalysisConnection
}

func newSolutionAgentKit(t *testing.T, optedIn bool) *solutionAgentKit {
	t.Helper()
	k := &solutionAgentKit{agentKit: newAgentKit(t, ""), features: []string{readonlyFeature, hubFeature}}
	k.agentKit.setting.MaxAgentRunsPerProject = 2
	k.conn = k.agentKit.conn
	k.settings = fakeEngineSettings{byProject: map[string]domain.ProjectEngineSettings{}}
	k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: fenced(domainTestdata(t, "solution_options/valid_with_design_references.json")), AppliedAccessMode: "readonly"}, nil
	}
	if optedIn {
		k.optIn()
	}
	return k
}

// optIn is applied lazily because the project id exists only once the Request is seeded.
func (k *solutionAgentKit) optIn() {
	k.settings.byProject["*"] = domain.ProjectEngineSettings{SolutionAgent: true}
}

func (k *solutionAgentKit) start(t *testing.T) (domain.AnalysisRun, error) {
	t.Helper()
	k.caps = fakeCaps{features: k.features}
	k.build()
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	if s, ok := k.settings.byProject["*"]; ok {
		s.ProjectID = req.ProjectID
		k.settings.byProject[req.ProjectID] = s
	}
	sp := &capturingSpawner{}
	gen := k.w.newGenerate(sp, fakeConns{conn: k.conn}).WithSolutionAgent(k.settings, k.caps)
	if _, err := gen.Execute(k.w.ctx(), GenerateSolutionInput{RequestID: req.ID}); err != nil {
		return domain.AnalysisRun{}, err
	}
	if len(sp.runs) != 1 {
		t.Fatalf("spawned %d runs", len(sp.runs))
	}
	return sp.runs[0], nil
}

func (k *solutionAgentKit) storedDesignReferences(t *testing.T, run domain.AnalysisRun) *domain.DesignReferences {
	t.Helper()
	opts, err := domain.ParseSolutionOptions(k.w.solutions[run.SolutionID].OptionsJSON)
	if err != nil {
		t.Fatal(err)
	}
	return opts.DesignReferences
}

func TestSolutionAgent_OptedInProjectGetsTheGraphAndKeepsItsReferences(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	if run.Mode != domain.AnalysisModeAgentReadonly {
		t.Fatalf("mode = %s", run.Mode)
	}
	k.runner.execute(run)
	done := k.w.runs[run.ID]
	if done.Status != domain.RunStatusSucceeded || done.RepoCheck != domain.RepoCheckClean || done.Enforcement != domain.EnforcementAgent {
		t.Fatalf("run = %s repo=%s enforcement=%s", describe(done), done.RepoCheck, done.Enforcement)
	}
	in := k.agent.inputs[0]
	if in.KnowledgeRoot != "/srv/repo" || in.ProposeChangesets || !strings.Contains(in.Prompt, "tdd-knowledge") || !strings.Contains(in.Prompt, "READ-ONLY") {
		t.Fatalf("agent input = %+v", in)
	}
	ref := k.storedDesignReferences(t, run)
	if ref == nil || !ref.Available || ref.Snapshot != "kg-2026-10-14" || len(ref.Read) != 2 || len(ref.Conflicts) != 1 {
		t.Fatalf("stored design_references = %+v", ref)
	}
	if k.comp.calls() != 0 {
		t.Fatal("an opted-in Solution must not go through ai.complete")
	}
}

func TestSolutionAgent_AReplyWithoutReferencesIsStoredAsUnreported(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: fenced(validSolutionReply(t)), AppliedAccessMode: "readonly"}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.runner.execute(run)
	ref := k.storedDesignReferences(t, run)
	if k.w.runs[run.ID].Status != domain.RunStatusSucceeded || ref == nil || ref.Available || len(ref.Gaps) != 1 {
		t.Fatalf("run=%s references=%+v", describe(k.w.runs[run.ID]), ref)
	}
}

func TestSolutionAgent_WhenTheGraphCannotBeLoadedTheSolutionStillComesBack(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	reply := fenced(domainTestdata(t, "solution_options/valid_with_design_references.json")) // the agent claims it read the graph
	k.agent.res = func(call int, in AgentPromptInput) (AgentPromptResult, error) {
		if in.KnowledgeRoot != "" {
			return AgentPromptResult{}, ErrAgentKnowledgeHubUnavailable
		}
		return AgentPromptResult{Stdout: reply, AppliedAccessMode: "readonly"}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.runner.execute(run)
	if k.w.runs[run.ID].Status != domain.RunStatusSucceeded || len(k.agent.inputs) != 2 {
		t.Fatalf("run=%s calls=%d", describe(k.w.runs[run.ID]), len(k.agent.inputs))
	}
	if second := k.agent.inputs[1]; second.KnowledgeRoot != "" || !strings.Contains(second.Prompt, "not available") {
		t.Fatalf("second call = %+v", second)
	}
	ref := k.storedDesignReferences(t, run)
	if ref == nil || ref.Available || len(ref.Read) != 0 || ref.Snapshot != "" || len(ref.Conflicts) != 0 {
		t.Fatalf("a run without the graph kept claims about it: %+v", ref)
	}
}

func TestSolutionAgent_StaysOnCompleteWhereTheAgentRouteIsNotAvailable(t *testing.T) {
	cases := map[string]func(*solutionAgentKit){
		"project did not opt in":        func(k *solutionAgentKit) { delete(k.settings.byProject, "*") },
		"agent cannot load the hub":     func(k *solutionAgentKit) { k.features = []string{readonlyFeature} },
		"capabilities unreadable":       func(k *solutionAgentKit) { k.features = nil },
		"no repository folder":          func(k *solutionAgentKit) { k.conn.RepoPath = "" },
		"settings cannot be read":       func(k *solutionAgentKit) { k.settings.err = errors.New("db down") },
		"opted in, repo has no hub yet": func(k *solutionAgentKit) { k.features = []string{"agent.execPrompt"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			k := newSolutionAgentKit(t, true)
			mutate(k)
			run, err := k.start(t)
			if err != nil {
				t.Fatal(err)
			}
			if run.Mode != domain.AnalysisModeComplete {
				t.Fatalf("mode = %s, want complete", run.Mode)
			}
		})
	}
}

func TestSolutionAgent_ACallerCannotForceTheAgentOnAProjectThatDidNotOptIn(t *testing.T) {
	k := newSolutionAgentKit(t, false)
	k.build()
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen := k.w.newGenerate(&capturingSpawner{}, fakeConns{conn: k.conn}).WithSolutionAgent(k.settings, fakeCaps{features: k.features})
	_, err := gen.Execute(k.w.ctx(), GenerateSolutionInput{RequestID: req.ID, Mode: domain.AnalysisModeAgentReadonly})
	if err == nil || !strings.Contains(err.Error(), "REQUEST_SOLUTION_MODE_NOT_ALLOWED") {
		t.Fatalf("err = %v, want REQUEST_SOLUTION_MODE_NOT_ALLOWED", err)
	}
}

func TestSettleDesignReferences_NeverLetsARunClaimAGraphItDidNotHave(t *testing.T) {
	doc := []byte(domainTestdata(t, "solution_options/valid_with_design_references.json"))
	out, err := settleDesignReferences(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		R map[string]any `json:"design_references"`
	}
	if err := json.Unmarshal(out, &m); err != nil || m.R["available"] != false || m.R["read"] != nil || m.R["snapshot"] != nil {
		t.Fatalf("settled = %s (%v)", out, err)
	}
	kept, err := settleDesignReferences(doc, true)
	if err != nil || !strings.Contains(string(kept), `"snapshot":"kg-2026-10-14"`) {
		t.Fatalf("a run with the graph lost its references: %s (%v)", kept, err)
	}
	if _, err := settleDesignReferences([]byte(`{`), true); err == nil {
		t.Fatal("garbage was accepted")
	}
}
