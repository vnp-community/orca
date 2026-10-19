package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Distinct, unlikely strings so a leak into a log or an event cannot be a coincidence.
var revisionAsks = []string{"ASK-ONE keep the current schema", "ASK-TWO add a rollback plan", "ASK-THREE shrink the rollout"}

// revisionDriver runs successive analysis rounds on one Request through the real GenerateSolution and the given worker.
type revisionDriver struct {
	t     *testing.T
	w     *solWorld
	gen   *GenerateSolution
	sp    *capturingSpawner
	exec  func(domain.AnalysisRun)
	reqID string
}

func (d *revisionDriver) round(feedback string) domain.AnalysisRun {
	d.t.Helper()
	before := len(d.sp.runs)
	if _, err := d.gen.Execute(d.w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: feedback}); err != nil {
		d.t.Fatalf("round with feedback %q: %v", feedback, err)
	}
	if len(d.sp.runs) != before+1 {
		d.t.Fatalf("round spawned %d runs", len(d.sp.runs)-before)
	}
	run := d.sp.runs[before]
	d.exec(run)
	got := d.w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded {
		d.t.Fatalf("run = %s", describe(got))
	}
	return got
}

// threeRevisions runs a first attempt and three requested revisions.
func (d *revisionDriver) threeRevisions() {
	d.t.Helper()
	d.round("")
	for _, ask := range revisionAsks {
		d.round(ask)
	}
}

// requireRevisionHistory checks the prompt of the last round: earlier asks sit in the prior_artifacts block in order, the
// newest sits in <feedback>, and nothing closed a block early.
func requireRevisionHistory(t *testing.T, prompt string) {
	t.Helper()
	prior := priorBlock(t, prompt)
	for _, ask := range revisionAsks[:2] {
		if !strings.Contains(prior, ask) {
			t.Errorf("earlier feedback %q is not inside <prior_artifacts>", ask)
		}
	}
	if strings.Contains(prior, revisionAsks[2]) || !strings.Contains(blockOf(t, prompt, "feedback"), revisionAsks[2]) {
		t.Errorf("the newest feedback belongs in <feedback> only")
	}
	if strings.Index(prior, revisionAsks[0]) > strings.Index(prior, revisionAsks[1]) {
		t.Error("earlier feedback must be listed oldest first")
	}
}

func requireListedRevisionFeedback(t *testing.T, w *solWorld, reqID string) {
	t.Helper()
	res := must(NewListSolutions(w, fakeSolutions{w}, fakeRuns{w}).Execute(w.ctx(), ListSolutionsInput{RequestID: reqID}))
	got := map[string]bool{}
	for _, s := range res.Solutions {
		if want := w.runs[s.GenerationRunID].Feedback; s.RevisionFeedback != want {
			t.Errorf("solution %s revision feedback = %q, want %q", s.ID, s.RevisionFeedback, want)
		}
		got[s.RevisionFeedback] = true
	}
	for _, ask := range revisionAsks {
		if !got[ask] {
			t.Errorf("no listed version carries %q", ask)
		}
	}
}

func completeDriver(t *testing.T, comp *scriptedCompleter) (*revisionDriver, *solWorld) {
	t.Helper()
	w := newSolWorld()
	runner, _ := newTestRunner(w, comp, nil)
	sp := &capturingSpawner{}
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	return &revisionDriver{t: t, w: w, gen: gen, sp: sp, exec: runner.execute, reqID: r.ID}, w
}

func TestRevisionFeedback_AiCompleteSolutionSeesEveryEarlierAsk(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.threeRevisions()

	if comp.calls() != 4 {
		t.Fatalf("completer calls = %d", comp.calls())
	}
	if strings.Contains(comp.prompts[0], "<prior_artifacts>") || strings.Contains(comp.prompts[0], "<feedback>") {
		t.Fatal("the first attempt has no history")
	}
	requireRevisionHistory(t, comp.prompts[3])
	requireListedRevisionFeedback(t, w, d.reqID)
}

func agentDriver(t *testing.T, k *agentKit, typ domain.RequestType, gen func(sp *capturingSpawner) *GenerateSolution) *revisionDriver {
	t.Helper()
	k.build()
	sp := &capturingSpawner{}
	r := k.w.seedRequest(typ, domain.RequestStatusAnalyzing)
	return &revisionDriver{t: t, w: k.w, gen: gen(sp), sp: sp, exec: k.runner.execute, reqID: r.ID}
}

func TestRevisionFeedback_AgentReadonlyAnalysisKindsSeeEveryEarlierAsk(t *testing.T) {
	for _, tc := range []struct {
		typ   domain.RequestType
		reply string
	}{
		{domain.RequestTypeBug, "diagnosis_valid.json"},
		{domain.RequestTypeSpike, "findings_valid.json"},
		{domain.RequestTypeQuestion, "answer_valid.json"},
	} {
		t.Run(string(tc.typ), func(t *testing.T) {
			k := newAgentKit(t, "")
			doc := analysisReply(t, tc.reply)
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: "```json\n" + doc + "\n```", AppliedAccessMode: "readonly"}, nil
			}
			d := agentDriver(t, k, tc.typ, func(sp *capturingSpawner) *GenerateSolution { return k.w.newGenerate(sp, fakeConns{conn: k.conn}) })
			d.threeRevisions()
			requireRevisionHistory(t, k.agent.inputs[len(k.agent.inputs)-1].Prompt)
			requireListedRevisionFeedback(t, k.w, d.reqID)
		})
	}
}

// With solution_agent on, the Solution of a change_request is written by the read-only agent with the knowledge graph.
func TestRevisionFeedback_AgentSolutionSeesEveryEarlierAsk(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	k.caps = fakeCaps{features: k.features}
	d := agentDriver(t, k.agentKit, domain.RequestTypeChangeRequest, func(*capturingSpawner) *GenerateSolution { return nil })
	req := k.w.requests[d.reqID]
	k.settings.byProject[req.ProjectID] = domain.ProjectEngineSettings{ProjectID: req.ProjectID, SolutionAgent: true}
	d.gen = k.w.newGenerate(d.sp, fakeConns{conn: k.conn}).WithSolutionAgent(k.settings, k.caps)
	d.threeRevisions()

	last := k.agent.inputs[len(k.agent.inputs)-1]
	if last.KnowledgeRoot == "" || !strings.Contains(last.Prompt, "design_references") {
		t.Fatalf("not the agent Solution prompt: %+v", last)
	}
	requireRevisionHistory(t, last.Prompt)
	if k.comp.calls() != 0 {
		t.Fatal("an opted-in Solution must not go through ai.complete")
	}
	requireListedRevisionFeedback(t, k.w, d.reqID)
}

// A dev server that cannot give the agent the knowledge graph keeps the Solution on ai.complete (the route the opt-in falls back to).
func TestRevisionFeedback_OptedInSolutionWithoutKnowledgeHubStillSeesEveryEarlierAsk(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	k.caps = fakeCaps{features: []string{readonlyFeature}}
	k.comp.replies = []string{validSolutionReply(t)}
	d := agentDriver(t, k.agentKit, domain.RequestTypeChangeRequest, func(*capturingSpawner) *GenerateSolution { return nil })
	req := k.w.requests[d.reqID]
	k.settings.byProject[req.ProjectID] = domain.ProjectEngineSettings{ProjectID: req.ProjectID, SolutionAgent: true}
	d.gen = k.w.newGenerate(d.sp, fakeConns{conn: k.conn}).WithSolutionAgent(k.settings, k.caps)
	d.threeRevisions()

	if len(k.agent.inputs) != 0 || k.comp.calls() != 4 {
		t.Fatalf("agent calls = %d, completer calls = %d", len(k.agent.inputs), k.comp.calls())
	}
	requireRevisionHistory(t, k.comp.prompts[3])
}

func TestRevisionFeedback_NativeEngineAnalysisPassesPriorFeedbackToThePrompt(t *testing.T) {
	var seen string
	eng := NewNativeEngine(completerFunc(func(_ context.Context, prompt string) (string, error) {
		seen = prompt
		return validSolutionReply(t), nil
	}), nil)
	_, err := eng.GenerateAnalysis(context.Background(), ProjectRef{}, AnalysisInput{
		Request:        sampleRequest("b"),
		PriorArtifacts: []PriorArtifact{{Kind: "solution", Status: "superseded", Content: "{}", Feedback: revisionAsks[0]}},
		Feedback:       revisionAsks[1],
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(priorBlock(t, seen), revisionAsks[0]) || !strings.Contains(blockOf(t, seen, "feedback"), revisionAsks[1]) {
		t.Fatalf("prompt lost the feedback:\n%s", seen)
	}
}

type completerFunc func(ctx context.Context, prompt string) (string, error)

func (f completerFunc) Complete(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func requireNoFeedbackText(t *testing.T, where, text string) {
	t.Helper()
	for _, ask := range revisionAsks {
		if strings.Contains(text, ask) {
			t.Errorf("%s contains the reviewer feedback %q", where, ask)
		}
	}
}

// The workers have no tracing; what they emit is logs, run errors and outbox events. None may carry the feedback, on the
// success path, when the feedback lookup fails, and when the model call fails.
func TestRevisionFeedback_NeverReachesLogsRunErrorsOrEvents(t *testing.T) {
	logs := captureLogs(t)
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.threeRevisions()

	w.failFeedback = errors.New("history store unavailable")
	comp.replies = []string{validSolutionReply(t)}
	d.round("ASK-FOUR while the history store is down")
	if !strings.Contains(logs.String(), "earlier reviewer feedback unreadable") {
		t.Fatalf("a failed feedback lookup must be logged:\n%s", logs)
	}
	w.failFeedback = nil
	prompt := comp.prompts[len(comp.prompts)-1]
	if strings.Contains(priorBlock(t, prompt), revisionAsks[0]) {
		t.Fatal("with the lookup down the history is shown without feedback, not with stale feedback")
	}

	comp.errs = []error{errors.New("model exploded")}
	failing := d.gen
	if _, err := failing.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-FIVE final"}); err != nil {
		t.Fatal(err)
	}
	d.exec(d.sp.runs[len(d.sp.runs)-1])

	requireNoFeedbackText(t, "logs", logs.String())
	for _, e := range w.events {
		requireNoFeedbackText(t, "outbox event "+e.Subject, string(e.Payload))
	}
	for _, run := range w.runs {
		if run.ErrorMessage != nil {
			requireNoFeedbackText(t, "run error message", *run.ErrorMessage)
		}
	}
	if strings.Contains(logs.String(), "ASK-FIVE") || strings.Contains(logs.String(), "ASK-FOUR") {
		t.Error("logs contain feedback text")
	}
}
