package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const goodTasksMD = `<!-- orca:begin plan request=REQ-42 schema=1 -->
## PH-1 Nền
- [ ] T1.1 Dựng client [type=feature] [h=2] [ac=AC-1]
  Serves: FR-1
  Code: create ` + "`a/b.go::X`" + `
  Verify: AC-1 via ` + "`go test`" + `
  Done when: xong.
<!-- orca:end plan -->
`

const changeTasksPath = "openspec/changes/req-42/tasks.md"

type engineKit struct {
	ws    *fakeWorkspace
	agent *fakeAgent
	e     *openspecEngine
	in    ProposalInput
}

func newEngineKit(features []string) *engineKit {
	ws, agent := newFakeWorkspace(), &fakeAgent{results: []AgentPromptResult{{ExitCode: 0}}}
	conns := fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}}
	gate := NewEngineReadinessGate(conns, fakeGateExec{res: ExecResult{Stdout: "1.3.0"}}, "1.0.0")
	e := NewOpenSpecEngine(OpenSpecEngineDeps{
		Gate: gate, Workspace: ws, Agent: agent, Conns: conns, Caps: specCaps{features: features}, NewNonce: func() string { return "nonce0123456789abcdef" },
	})
	req := domain.Request{ID: "req-uuid", ProjectID: "proj", Number: 42, Title: "Add AI", Type: domain.RequestTypeChangeRequest, Size: domain.RequestSizeL}
	in := ProposalInput{Run: domain.AnalysisRun{ID: "run-1", TenantID: "t1"}, Request: req, Shape: domain.ShapePlanPhases, Limits: domain.DefaultPlanLimits}
	return &engineKit{ws: ws, agent: agent, e: e, in: in}
}

func (k *engineKit) agentWritesTasks(md string) {
	k.agent.onCall = func(int, AgentPromptInput) { k.ws.write(changeTasksPath, md) }
}

func failureOf(t *testing.T, err error) *AnalysisFailure {
	t.Helper()
	f, ok := asFailure(err)
	if !ok {
		t.Fatalf("want an *AnalysisFailure, got %v", err)
	}
	return f
}

func TestOpenSpecEngine_AuthorsTheChangeInAFreshWorktree(t *testing.T) {
	k := newEngineKit(append(authoringFeatures, "agent.execPrompt.knowledgeHub"))
	k.agentWritesTasks(goodTasksMD)
	k.agent.results = []AgentPromptResult{{ResultBlock: []byte(`{"changeId":"req-42","phases":1,"tasks":1,"validation":"passed","knowledge":{"available":true,"snapshot":"snap-1","read":["FR-1"]}}`)}}
	res, err := k.e.GenerateProposal(context.Background(), k.in)
	if err != nil {
		t.Fatal(err)
	}
	if res.ChangeID != "req-42" || res.Ref != "REQ-42" || res.TasksMD != goodTasksMD || res.Workspace.WorktreeID != "wt-1" {
		t.Fatalf("result = %+v", res)
	}
	if !res.Report.Present || res.Report.Knowledge.Snapshot != "snap-1" || len(res.Report.Knowledge.Read) != 1 || !res.KnowledgeRequested {
		t.Fatalf("report = %+v", res.Report)
	}
	if k.ws.ensured != 1 || k.ws.discarded != 1 {
		t.Fatalf("a fresh run takes a clean worktree: ensured=%d discarded=%d", k.ws.ensured, k.ws.discarded)
	}
	in := k.agent.inputs[0]
	if in.WriteProfile != "openspec-author" || in.RepoPath != "/srv/wt-1" || in.ResultNonce != "nonce0123456789abcdef" || in.ReadOnlyEnforced {
		t.Fatalf("agent input = %+v", in)
	}
	if in.KnowledgeRoot != "/srv/repo" {
		t.Fatalf("the graph lives in the project checkout, not in the proposal worktree: %q", in.KnowledgeRoot)
	}
	if !strings.Contains(in.Prompt, "openspec/changes/req-42/") || !strings.Contains(in.Prompt, "request=REQ-42") || !strings.Contains(in.Prompt, "ORCA_RESULT_BEGIN nonce0123456789abcdef") {
		t.Fatalf("prompt: %s", in.Prompt)
	}
	if in.TimeoutMS != 900_000 || in.ProposeChangesets {
		t.Fatalf("timeout %d propose %v", in.TimeoutMS, in.ProposeChangesets)
	}
}

func TestOpenSpecEngine_ARetryReusesTheWorktreeAndKeepsTheFiles(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.agentWritesTasks(goodTasksMD)
	first, err := k.e.GenerateProposal(context.Background(), k.in)
	if err != nil {
		t.Fatal(err)
	}
	k.in.Workspace, k.in.RetryNote = &first.Workspace, "tasks.md line 5 TASKSMD_UNKNOWN_LINE"
	if _, err := k.e.GenerateProposal(context.Background(), k.in); err != nil {
		t.Fatal(err)
	}
	if k.ws.ensured != 1 || k.ws.discarded != 1 {
		t.Fatalf("the retry must not recreate or clean the worktree: ensured=%d discarded=%d", k.ws.ensured, k.ws.discarded)
	}
	if !strings.Contains(k.agent.inputs[1].Prompt, "TASKSMD_UNKNOWN_LINE") || strings.Contains(k.agent.inputs[0].Prompt, "TASKSMD_UNKNOWN_LINE") {
		t.Fatal("only the second prompt carries the findings")
	}
}

func TestOpenSpecEngine_WithoutTheKnowledgeFeatureTheAgentGetsNoRoot(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.agentWritesTasks(goodTasksMD)
	res, err := k.e.GenerateProposal(context.Background(), k.in)
	if err != nil || res.KnowledgeRequested || k.agent.inputs[0].KnowledgeRoot != "" {
		t.Fatalf("%+v %v", k.agent.inputs[0], err)
	}
	if !strings.Contains(k.agent.inputs[0].Prompt, "No design knowledge graph is available") {
		t.Fatal("the prompt must say the graph is absent")
	}
}

func TestOpenSpecEngine_AMissingGraphFallsBackToARunWithoutIt(t *testing.T) {
	k := newEngineKit(append(authoringFeatures, "agent.execPrompt.knowledgeHub"))
	k.agentWritesTasks(goodTasksMD)
	k.agent.errs = []error{ErrAgentKnowledgeHubUnavailable}
	res, err := k.e.GenerateProposal(context.Background(), k.in)
	if err != nil {
		t.Fatal(err)
	}
	if k.agent.calls() != 2 || k.agent.inputs[0].KnowledgeRoot == "" || k.agent.inputs[1].KnowledgeRoot != "" || res.KnowledgeRequested {
		t.Fatalf("calls=%d inputs=%+v", k.agent.calls(), k.agent.inputs)
	}
	if strings.Contains(k.agent.inputs[1].Prompt, "MCP server tdd-agent") {
		t.Fatal("the second prompt must not send the agent to a graph that is not there")
	}
}

func TestOpenSpecEngine_RefusesAnAgentThatCannotRunItSafely(t *testing.T) {
	cases := map[string][]string{
		"no profile feature": {"agent.execPrompt", "agent.execPrompt.workspaceKind", "agent.execPrompt.changes", "agent.execPrompt.resultBlock"},
		"no changes report":  {"agent.execPrompt", "agent.execPrompt.workspaceKind", "agent.execPrompt.resultBlock", "agent.execPrompt.permissionProfile"},
		"nothing":            nil,
	}
	for name, features := range cases {
		k := newEngineKit(features)
		_, err := k.e.GenerateProposal(context.Background(), k.in)
		if f := failureOf(t, err); f.Code != domain.RunErrSpecAgentOld || k.agent.calls() != 0 || k.ws.ensured != 0 {
			t.Fatalf("%s: %+v calls=%d ensured=%d", name, f, k.agent.calls(), k.ws.ensured)
		}
	}
	k := newEngineKit(authoringFeatures)
	k.e.d.Caps = nil
	if f := failureOf(t, errOf(k.e.GenerateProposal(context.Background(), k.in))); f.Code != domain.RunErrSpecAgentOld {
		t.Fatalf("no capability reader: %+v", f)
	}
	k.e.d.Caps = specCaps{err: errors.New("infra down")}
	if f := failureOf(t, errOf(k.e.GenerateProposal(context.Background(), k.in))); f.Code != domain.RunErrSpecAgentOld {
		t.Fatalf("unreadable capabilities must not be assumed safe: %+v", f)
	}
}

func errOf(_ ProposalResult, err error) error { return err }

func TestOpenSpecEngine_StopsBeforeTheAgentWhenTheServerIsNotReady(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.e.gate = NewEngineReadinessGate(fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/r"}}, fakeGateExec{res: ExecResult{ExitCode: -1, SpawnError: "ENOENT"}}, "")
	f := failureOf(t, errOf(k.e.GenerateProposal(context.Background(), k.in)))
	if f.Code != "REQUEST_ENGINE_OPENSPEC_MISSING" || k.agent.calls() != 0 || k.ws.ensured != 0 {
		t.Fatalf("%+v calls=%d ensured=%d", f, k.agent.calls(), k.ws.ensured)
	}
}

func TestOpenSpecEngine_FailureCodes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(k *engineKit)
		code  string
	}{
		{"agent timed out", func(k *engineKit) { k.agent.results = []AgentPromptResult{{TimedOut: true, ExitCode: -1}} }, domain.RunErrSpecTimeout},
		{"agent exits non-zero", func(k *engineKit) { k.agent.results = []AgentPromptResult{{ExitCode: 1, Stderr: "boom"}} }, domain.RunErrSpecAgentFailed},
		{"relay deadline", func(k *engineKit) { k.agent.errs = []error{context.DeadlineExceeded} }, domain.RunErrSpecTimeout},
		{"relay breaks", func(k *engineKit) { k.agent.errs = []error{errors.New("boom")} }, domain.RunErrSpecAgentFailed},
		{"no dev server for the agent", func(k *engineKit) { k.agent.errs = []error{ErrNoDevServer} }, domain.RunErrAnalysisNoConn},
		{"tasks.md missing", func(k *engineKit) { k.agent.onCall = nil }, domain.RunErrSpecInvalidOutput},
		{"agent wrote outside the change", func(k *engineKit) {
			k.agent.onCall = func(int, AgentPromptInput) {
				k.ws.write(changeTasksPath, goodTasksMD)
				k.ws.write("backend/main.go", "package main")
			}
		}, domain.RunErrSpecOutOfScope},
		{"agent renamed a repo file into the change", func(k *engineKit) {
			k.agent.onCall = func(int, AgentPromptInput) {
				k.ws.write(changeTasksPath, goodTasksMD)
				k.ws.changed = append(k.ws.changed, ChangedPath{Path: "openspec/changes/req-42/x.md", OldPath: "src/secret.go", State: "renamed"})
			}
		}, domain.RunErrSpecOutOfScope},
		{"workspace cannot be created", func(k *engineKit) { k.ws.ensureErr = domain.ErrEngineRepoNotResolved }, "REQUEST_ENGINE_REPO_NOT_RESOLVED"},
		{"workspace cannot be cleaned", func(k *engineKit) { k.ws.discErr = errors.New("busy") }, domain.RunErrSpecAgentFailed},
		{"status unreadable after the run", func(k *engineKit) {
			k.agent.onCall = func(int, AgentPromptInput) {
				k.ws.write(changeTasksPath, goodTasksMD)
				k.ws.changeErr = errors.New("git down")
			}
		}, domain.RunErrSpecAgentFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := newEngineKit(authoringFeatures)
			tc.setup(k)
			_, err := k.e.GenerateProposal(context.Background(), k.in)
			if f := failureOf(t, err); f.Code != tc.code {
				t.Fatalf("code = %s (%s), want %s", f.Code, f.Message, tc.code)
			}
		})
	}
}

func TestOpenSpecEngine_OutOfScopeKeepsTheWorktreeAndNamesThePaths(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.agent.onCall = func(int, AgentPromptInput) {
		k.ws.write(changeTasksPath, goodTasksMD)
		k.ws.write("backend/main.go", "x")
	}
	_, err := k.e.GenerateProposal(context.Background(), k.in)
	f := failureOf(t, err)
	if !strings.Contains(f.Message, "backend/main.go") || k.ws.removed != 0 || k.ws.discarded != 1 {
		t.Fatalf("%+v removed=%d discarded=%d (the one discard is the fresh start)", f, k.ws.removed, k.ws.discarded)
	}
}

func TestOpenSpecEngine_NoRepoPathOrDevServer(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.e.d.Conns = fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}
	if f := failureOf(t, errOf(k.e.GenerateProposal(context.Background(), k.in))); f.Code != domain.RunErrNoRepoPath {
		t.Fatalf("%+v", f)
	}
	k.e.d.Conns = fakeConns{err: ErrNoDevServer}
	if f := failureOf(t, errOf(k.e.GenerateProposal(context.Background(), k.in))); f.Code != domain.RunErrAnalysisNoConn {
		t.Fatalf("%+v", f)
	}
	boom := errors.New("resolver down")
	k.e.d.Conns = fakeConns{err: boom}
	if _, err := k.e.GenerateProposal(context.Background(), k.in); !errors.Is(err, boom) {
		t.Fatalf("an unknown resolver error passes through untouched: %v", err)
	}
}

func TestOpenSpecEngine_ReportIsDataNotTrust(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	k.agentWritesTasks(goodTasksMD)
	for _, block := range [][]byte{nil, []byte("not json"), []byte(`[1,2]`)} {
		k.agent.results = []AgentPromptResult{{ResultBlock: block}}
		res, err := k.e.GenerateProposal(context.Background(), k.in)
		if err != nil || res.Report.Present {
			t.Fatalf("a missing or broken block is not an error and leaves the report empty: %+v %v", res.Report, err)
		}
	}
}

func TestOpenSpecEngine_StepsItDoesNotOwn(t *testing.T) {
	k := newEngineKit(authoringFeatures)
	if _, err := k.e.GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{}); !errors.Is(err, ErrEngineStepNotSupported) {
		t.Fatalf("a solution must not come back empty and silent: %v", err)
	}
	if _, err := k.e.GeneratePlan(context.Background(), ProjectRef{}, PlanInput{}); !errors.Is(err, ErrEngineStepNotSupported) {
		t.Fatalf("%v", err)
	}
	if err := k.e.OnRequestCompleted(context.Background(), ProjectRef{}, CompletionInput{}); err != nil {
		t.Fatal(err)
	}
	if k.e.Name() != domain.EngineOpenSpec {
		t.Fatal("name")
	}
}

func TestRandomNonceFitsTheAgentsRule(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		n := randomNonce()
		if len(n) < 16 || len(n) > 64 || strings.Trim(n, "0123456789abcdef") != "" || seen[n] {
			t.Fatalf("bad or repeated nonce %q", n)
		}
		seen[n] = true
	}
}
