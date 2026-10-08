package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const readonlyFeature = "agent.execPrompt.readonly"

type agentKit struct {
	w       *solWorld
	agent   *scriptedAgent
	probe   *scriptedProbe
	runner  *AnalysisRunner
	comp    *scriptedCompleter
	conn    AnalysisConnection
	caps    DevServerCapabilityReader
	setting AnalysisSettings
}

func newAgentKit(t *testing.T, reply string) *agentKit {
	t.Helper()
	k := &agentKit{
		w:       newSolWorld(),
		comp:    &scriptedCompleter{replies: []string{"must not be called"}},
		probe:   &scriptedProbe{snaps: []RepoSnapshot{{Branch: "main", Files: []RepoFileState{{Path: "a.go", State: "modified"}}}}},
		conn:    AnalysisConnection{ConnectionID: "c1", RepoPath: "/srv/repo", WorktreeID: "wt-1"},
		caps:    fakeCaps{features: []string{readonlyFeature}},
		setting: func() AnalysisSettings { d := DefaultAnalysisSettings(); d.Heartbeat = time.Hour; return d }(),
	}
	k.agent = &scriptedAgent{res: func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: reply, AppliedAccessMode: "readonly"}, nil
	}}
	return k
}

func (k *agentKit) build() {
	w := k.w
	worker := NewRunAgentReadonlyAnalysis(AgentReadonlyDeps{
		Requests: w, Solutions: fakeSolutions{w}, Conns: fakeConns{conn: k.conn}, Agent: k.agent, Probe: k.probe, Caps: k.caps,
		Writer: w.newWriter(w.transitioner()), Settings: k.setting,
	})
	gen := NewRunSolutionGeneration(w, fakeSolutions{w}, k.comp, nil, w.newWriter(w.transitioner()), k.setting)
	k.runner = NewAnalysisRunner(fakeRuns{w}, fakeSolutions{w}, w, gen, worker, k.setting, testOwner)
}

// run starts and executes one analysis for a request of the given type.
func (k *agentKit) run(t *testing.T, typ domain.RequestType) (domain.AnalysisRun, domain.Request) {
	t.Helper()
	k.build()
	run, r := startRun(t, k.w, typ, k.conn)
	k.runner.execute(run)
	return k.w.runs[run.ID], k.w.requests[r.ID]
}

func TestAgentReadonly_SuccessPerKindOpensTheRightApproval(t *testing.T) {
	cases := []struct {
		typ     domain.RequestType
		reply   string
		subject domain.SubjectType
		kind    domain.SolutionKind
		status  domain.RequestStatus
	}{
		{domain.RequestTypeBug, "diagnosis_valid.json", domain.SubjectSolution, domain.SolutionKindDiagnosis, domain.RequestStatusAwaitingAnalysisApproval},
		{domain.RequestTypeSpike, "findings_valid.json", domain.SubjectFindings, domain.SolutionKindFindings, domain.RequestStatusAwaitingAnalysisApproval},
		{domain.RequestTypeQuestion, "answer_valid.json", domain.SubjectAnswer, domain.SolutionKindAnswer, domain.RequestStatusAwaitingAnalysisApproval},
	}
	for _, tc := range cases {
		t.Run(string(tc.typ), func(t *testing.T) {
			k := newAgentKit(t, "")
			doc := analysisReply(t, tc.reply)
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: "```json\n" + doc + "\n```", AppliedAccessMode: "readonly"}, nil
			}
			run, req := k.run(t, tc.typ)
			if run.Status != domain.RunStatusSucceeded || run.Enforcement != domain.EnforcementAgent || run.RepoCheck != domain.RepoCheckClean {
				t.Fatalf("run = %s enforcement=%s repo=%s", describe(run), run.Enforcement, run.RepoCheck)
			}
			sol := k.w.solutions[run.SolutionID]
			if sol.Kind != tc.kind || sol.Status != domain.SolutionStatusProposed || sol.ChosenOption != nil {
				t.Fatalf("solution = %+v", sol)
			}
			var stored map[string]any
			if err := json.Unmarshal(sol.OptionsJSON, &stored); err != nil || stored["kind"] != string(tc.kind) {
				t.Fatalf("stored document = %s (%v)", sol.OptionsJSON, err)
			}
			if len(k.w.opened) != 1 || k.w.opened[0].SubjectType != tc.subject {
				t.Fatalf("approvals = %+v", k.w.opened)
			}
			if req.Status != tc.status {
				t.Fatalf("request = %s", req.Status)
			}
			if k.comp.calls() != 0 {
				t.Fatal("agent analysis must never fall back to ai.complete")
			}
		})
	}
}

func TestAgentReadonly_InputIsReadonlyOnTheRepoWithoutCreatingWorktrees(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	run, _ := k.run(t, domain.RequestTypeQuestion)
	if len(k.agent.inputs) != 1 {
		t.Fatalf("agent calls = %d", len(k.agent.inputs))
	}
	in := k.agent.inputs[0]
	if in.RepoPath != "/srv/repo" || in.StepID != run.ID || !in.ReadOnlyEnforced || in.TimeoutMS != 600_000 {
		t.Fatalf("input = %+v", in)
	}
	if !strings.Contains(in.Prompt, "READ-ONLY") || !strings.Contains(in.Prompt, "<request>") {
		t.Fatalf("prompt lacks the read-only preamble or the fenced request:\n%s", in.Prompt)
	}
	// AgentPromptInput has no trust preset or env field, so full trust cannot be requested; probes only ever read.
	if k.probe.calls != 2 {
		t.Fatalf("probe calls = %d, want before and after", k.probe.calls)
	}
}

func TestAgentReadonly_OldAgentFallsBackToPromptOnlyAndRecordsIt(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	k.caps = fakeCaps{features: []string{"agent.execPrompt"}}
	run, _ := k.run(t, domain.RequestTypeQuestion)
	if run.Status != domain.RunStatusSucceeded || run.Enforcement != domain.EnforcementPromptOnly || k.agent.inputs[0].ReadOnlyEnforced {
		t.Fatalf("run=%s enforcement=%s input=%+v", describe(run), run.Enforcement, k.agent.inputs[0])
	}
	// An unreadable profile is treated the same way, never as enforced.
	for _, err := range []error{ErrCapabilityNotAvailable, errors.New("infra down")} {
		k2 := newAgentKit(t, analysisReply(t, "answer_valid.json"))
		k2.caps = fakeCaps{err: err}
		if r, _ := k2.run(t, domain.RequestTypeQuestion); r.Enforcement != domain.EnforcementPromptOnly {
			t.Fatalf("enforcement = %s for %v", r.Enforcement, err)
		}
	}
}

func TestAgentReadonly_RequireEnforcedRejectsOldAgents(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	k.caps = fakeCaps{features: nil}
	k.setting.RequireEnforcedReadonly = true
	run, req := k.run(t, domain.RequestTypeQuestion)
	assertFailed(t, k.w, run, req, domain.RunErrAgentTooOld)
	if len(k.agent.inputs) != 0 {
		t.Fatal("agent must not run")
	}
}

func TestAgentReadonly_FlagOffForcesPromptOnly(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	k.setting.UseAgentReadonlyFlag = false
	run, _ := k.run(t, domain.RequestTypeQuestion)
	if run.Enforcement != domain.EnforcementPromptOnly || k.agent.inputs[0].ReadOnlyEnforced {
		t.Fatalf("flag off must not send accessMode: %s %+v", run.Enforcement, k.agent.inputs[0])
	}
}

func assertFailed(t *testing.T, w *solWorld, run domain.AnalysisRun, req domain.Request, code string) {
	t.Helper()
	if run.Status != domain.RunStatusFailed || run.ErrorCode == nil || *run.ErrorCode != code {
		t.Fatalf("run = %s, want failed %s", describe(run), code)
	}
	if len(w.solutions) != 0 {
		t.Fatalf("draft or result left behind: %+v", w.solutions)
	}
	if req.Status != domain.RequestStatusAnalyzing || len(w.opened) != 0 {
		t.Fatalf("request=%s approvals=%d", req.Status, len(w.opened))
	}
}

func TestAgentReadonly_AgentFailuresFailTheRun(t *testing.T) {
	cases := map[string]struct {
		res  AgentPromptResult
		err  error
		code string
	}{
		"exit code":   {res: AgentPromptResult{ExitCode: 2, Stdout: "{}"}, code: domain.RunErrAnalysisAgent},
		"timed out":   {res: AgentPromptResult{TimedOut: true}, code: domain.RunErrAnalysisTimeout},
		"call error":  {err: errors.New("relay down"), code: domain.RunErrAnalysisAgent},
		"deadline":    {err: context.DeadlineExceeded, code: domain.RunErrAnalysisTimeout},
		"no server":   {err: ErrNoDevServer, code: domain.RunErrAnalysisNoConn},
		"refused":     {err: ErrAgentReadonlyUnsupported, code: domain.RunErrAgentTooOld},
		"not applied": {res: AgentPromptResult{Stdout: "x", AppliedAccessMode: "write"}, code: domain.RunErrReadonlyNotActive},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := newAgentKit(t, "")
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) { return tc.res, tc.err }
			run, req := k.run(t, domain.RequestTypeQuestion)
			assertFailed(t, k.w, run, req, tc.code)
		})
	}
}

func TestAgentReadonly_InvalidJSONRetriesOnceThenFails(t *testing.T) {
	k := newAgentKit(t, "không phải JSON")
	run, req := k.run(t, domain.RequestTypeQuestion)
	assertFailed(t, k.w, run, req, domain.RunErrAnalysisInvalid)
	if len(k.agent.inputs) != 2 || !strings.Contains(k.agent.inputs[1].Prompt, "no JSON object") {
		t.Fatalf("agent calls = %d, want one retry carrying the error", len(k.agent.inputs))
	}

	k = newAgentKit(t, "")
	good := analysisReply(t, "answer_valid.json")
	k.agent.res = func(n int, _ AgentPromptInput) (AgentPromptResult, error) {
		if n == 0 {
			return AgentPromptResult{Stdout: `{"schema_version":1,"kind":"answer","answer_markdown":"","confidence":0.5}`, AppliedAccessMode: "readonly"}, nil
		}
		return AgentPromptResult{Stdout: good, AppliedAccessMode: "readonly"}, nil
	}
	run, _ = k.run(t, domain.RequestTypeQuestion)
	if run.Status != domain.RunStatusSucceeded || run.Attempt != 2 {
		t.Fatalf("run = %s attempt=%d", describe(run), run.Attempt)
	}
}

func TestAgentReadonly_RepoChangeDiscardsTheResult(t *testing.T) {
	cases := map[string]func(k *agentKit){
		"snapshot differs after the run": func(k *agentKit) {
			k.probe.snaps = []RepoSnapshot{{Branch: "main"}, {Branch: "main", Files: []RepoFileState{{Path: "new.txt", State: "untracked"}}}}
		},
		"branch changed": func(k *agentKit) {
			k.probe.snaps = []RepoSnapshot{{Branch: "main"}, {Branch: "feature"}}
		},
		"agent warns READONLY_VIOLATION": func(k *agentKit) {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: analysisReply(t, "answer_valid.json"), AppliedAccessMode: "readonly", Warnings: []string{"READONLY_VIOLATION"}}, nil
			}
		},
		"agent reports changed files": func(k *agentKit) {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: analysisReply(t, "answer_valid.json"), AppliedAccessMode: "readonly", ChangedFiles: 1}, nil
			}
		},
		"head moved": func(k *agentKit) {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: analysisReply(t, "answer_valid.json"), AppliedAccessMode: "readonly", HeadMoved: true}, nil
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
			setup(k)
			run, req := k.run(t, domain.RequestTypeQuestion)
			assertFailed(t, k.w, run, req, domain.RunErrRepoModified)
			if run.RepoCheck != domain.RepoCheckModified {
				t.Fatalf("repo_check = %s", run.RepoCheck)
			}
		})
	}
}

func TestAgentReadonly_FileOrderInSnapshotsDoesNotMatter(t *testing.T) {
	a := RepoSnapshot{Branch: "main", Files: []RepoFileState{{"a", "modified"}, {"b", "added"}}}
	b := RepoSnapshot{Branch: "main", Files: []RepoFileState{{"b", "added"}, {"a", "modified"}}}
	if !a.Equal(b) || !b.Equal(a) {
		t.Fatal("same files in another order must be equal")
	}
	if a.Equal(RepoSnapshot{Branch: "main", Files: []RepoFileState{{"a", "modified"}, {"b", "deleted"}}}) {
		t.Fatal("different state must differ")
	}
	if a.Equal(RepoSnapshot{Branch: "main", Files: []RepoFileState{{"a", "modified"}}}) {
		t.Fatal("different length must differ")
	}
	if (RepoSnapshot{Files: []RepoFileState{{"a", "m"}, {"a", "m"}}}).Equal(RepoSnapshot{Files: []RepoFileState{{"a", "m"}, {"b", "m"}}}) {
		t.Fatal("multiset comparison expected")
	}
}

func TestAgentReadonly_NoWorktreeIdRecordsSkippedAndStillSucceeds(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	k.conn.WorktreeID = ""
	run, _ := k.run(t, domain.RequestTypeQuestion)
	if run.Status != domain.RunStatusSucceeded || run.RepoCheck != domain.RepoCheckSkipped {
		t.Fatalf("run = %s repo_check=%s", describe(run), run.RepoCheck)
	}
	if k.probe.calls != 0 {
		t.Fatal("no probe call is possible without a worktree id")
	}
}

func TestAgentReadonly_ProbeOutageSkipsInsteadOfFailing(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "answer_valid.json"))
	k.probe.err = errors.New("git-gateway down")
	run, _ := k.run(t, domain.RequestTypeQuestion)
	if run.Status != domain.RunStatusSucceeded || run.RepoCheck != domain.RepoCheckSkipped {
		t.Fatalf("run = %s repo_check=%s", describe(run), run.RepoCheck)
	}
}

func TestAgentReadonly_NoRepoPathOrConnectionFails(t *testing.T) {
	k := newAgentKit(t, "")
	k.conn.RepoPath = ""
	k.build()
	run, req := startRunWith(t, k, domain.RequestTypeQuestion, AnalysisConnection{ConnectionID: "c", RepoPath: "/x"})
	k.runner.execute(run)
	assertFailed(t, k.w, k.w.runs[run.ID], k.w.requests[req.ID], domain.RunErrNoRepoPath)
}

func startRunWith(t *testing.T, k *agentKit, typ domain.RequestType, conn AnalysisConnection) (domain.AnalysisRun, domain.Request) {
	return startRun(t, k.w, typ, conn)
}

func TestAgentReadonly_HotfixAutoApprovesWithoutAnApproval(t *testing.T) {
	k := newAgentKit(t, analysisReply(t, "diagnosis_valid.json"))
	run, req := k.run(t, domain.RequestTypeHotfix)
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %s", describe(run))
	}
	sol := k.w.solutions[run.SolutionID]
	if sol.Status != domain.SolutionStatusApproved || len(k.w.opened) != 0 {
		t.Fatalf("solution=%s approvals=%d", sol.Status, len(k.w.opened))
	}
	if req.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s", req.Status)
	}
	if k.agent.inputs[0].TimeoutMS != 300_000 {
		t.Fatalf("hotfix timeout = %d", k.agent.inputs[0].TimeoutMS)
	}
	var auto map[string]any
	for _, e := range k.w.events {
		if e.Subject == domain.SubjectSolutionApproved {
			_ = json.Unmarshal(e.Payload, &auto)
		}
	}
	if auto["auto"] != true || auto["mode"] != "agent_readonly" || auto["solution_id"] != sol.ID {
		t.Fatalf("approved event payload = %v", auto)
	}
}

func TestAgentReadonly_SecretsAreRedactedInDocumentAndRawOutput(t *testing.T) {
	k := newAgentKit(t, withLeaks(analysisReply(t, "diagnosis_valid.json")))
	run, _ := k.run(t, domain.RequestTypeBug)
	if run.Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %s", describe(run))
	}
	for name, text := range map[string]string{"options": string(k.w.solutions[run.SolutionID].OptionsJSON), "raw_output": *run.RawOutput} {
		if strings.Contains(text, "BEGIN RSA PRIVATE KEY") || strings.Contains(text, "ghp_abcdefghij") {
			t.Errorf("%s still holds a secret: %s", name, text)
		}
		if !strings.Contains(text, "[REDACTED]") {
			t.Errorf("%s lacks the redaction marker", name)
		}
	}
}

func TestAgentReadonly_EscalationHintNeverChangesTheType(t *testing.T) {
	doc := setBool(analysisReply(t, "diagnosis_valid.json"), "suggest_escalate_to_change_request", true)
	k := newAgentKit(t, doc)
	run, req := k.run(t, domain.RequestTypeBug)
	if run.Status != domain.RunStatusSucceeded || req.Type != domain.RequestTypeBug {
		t.Fatalf("run=%s type=%s", describe(run), req.Type)
	}
	if !strings.Contains(string(k.w.solutions[run.SolutionID].OptionsJSON), `"suggest_escalate_to_change_request":true`) {
		t.Fatal("the hint must still be shown")
	}
}

func TestAgentReadonly_CompleteModeRoutesToTheCompleter(t *testing.T) {
	w := newSolWorld()
	comp := &scriptedCompleter{replies: []string{analysisReply(t, "answer_valid.json")}}
	agent := &scriptedAgent{res: func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{}, errors.New("must not run")
	}}
	settings := AnalysisSettings{}
	worker := NewRunAgentReadonlyAnalysis(AgentReadonlyDeps{Requests: w, Solutions: fakeSolutions{w}, Conns: fakeConns{}, Agent: agent, Writer: w.newWriter(w.transitioner()), Settings: settings})
	gen := NewRunSolutionGeneration(w, fakeSolutions{w}, comp, nil, w.newWriter(w.transitioner()), settings)
	runner := NewAnalysisRunner(fakeRuns{w}, fakeSolutions{w}, w, gen, worker, settings, testOwner)
	sp := &capturingSpawner{}
	r := w.seedRequest(domain.RequestTypeQuestion, domain.RequestStatusAnalyzing)
	if _, err := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Mode: domain.AnalysisModeComplete}); err != nil {
		t.Fatal(err)
	}
	runner.execute(sp.runs[0])
	if w.runs[sp.runs[0].ID].Status != domain.RunStatusSucceeded || comp.calls() != 1 || len(agent.inputs) != 0 {
		t.Fatalf("run=%s completer=%d agent=%d", describe(w.runs[sp.runs[0].ID]), comp.calls(), len(agent.inputs))
	}
}
