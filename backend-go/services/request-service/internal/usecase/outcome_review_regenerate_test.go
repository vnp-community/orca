package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type regenEnv struct {
	*drawEnv
	regen *ReviewRegenerator
}

func newRegenEnv(t *testing.T) *regenEnv {
	t.Helper()
	e := &regenEnv{drawEnv: newDrawEnv(t, domain.RequestStatusExecuting)}
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	e.start(t, StartDrawInput{}) // the review has its two drawings
	e.svc.Artifacts, e.svc.Runs = e.store, e.runs
	e.runner.Draw = e.draw
	e.regen = &ReviewRegenerator{Service: e.svc, Context: e.runner, Draw: e.draw, Runs: e.runs, Artifacts: e.store, Contexts: e.contexts, Config: e.runner.Config}
	return e
}

func (e *regenEnv) req2(scope domain.RegenScope, sections ...domain.ReviewSection) RegenerateInput {
	return RegenerateInput{RequestID: e.req.ID, RegenRequest: domain.RegenRequest{Scope: scope, Sections: sections, Reason: domain.RegenLooksWrong}, By: "user-1"}
}

func (e *regenEnv) revisions(t *testing.T, section domain.ReviewSection, name string) []domain.ReviewArtifact {
	t.Helper()
	r, _ := e.artifacts.ListRevisions(e.ctx, e.req.ID, section, domain.DiagramVariantDesign, name)
	return r
}

func TestRegenerate_ArtifactScopeDrawsOnlyThatSectionAndMakesOneRevision(t *testing.T) {
	e := newRegenEnv(t)
	flowBefore := e.revisions(t, domain.SectionFlow, "Bug status")
	archBefore := e.revisions(t, domain.SectionArchitecture, "")
	callsBefore := e.drawAgent.count()

	in := e.req2(domain.RegenArtifact, domain.SectionFlow)
	in.Note = "the arrows look reversed"
	res, err := e.regen.Regenerate(e.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.DrawRun == nil || res.ContextRun != nil || e.drawAgent.count() != callsBefore+1 {
		t.Fatalf("one draw run: %+v calls=%d", res, e.drawAgent.count())
	}
	if prompt := e.drawAgent.calls[callsBefore].Prompt; strings.Count(prompt, "<diagram ") != 1 || !strings.Contains(prompt, `section="flow"`) {
		t.Errorf("only the flow is drawn:\n%s", prompt)
	}
	flow := e.revisions(t, domain.SectionFlow, "Bug status")
	if len(flow) != len(flowBefore)+1 || flow[0].State != domain.ArtifactActive || flow[0].Origin != domain.ArtifactOriginRegenerated || flow[0].Reason != "looks_wrong" || flow[0].Revision != 2 {
		t.Fatalf("flow revisions: %+v", flow)
	}
	if flow[1].State != domain.ArtifactSuperseded {
		t.Errorf("the old revision stays and is superseded: %+v", flow[1])
	}
	if arch := e.revisions(t, domain.SectionArchitecture, ""); len(arch) != len(archBefore) || arch[0].State != domain.ArtifactActive {
		t.Errorf("the architecture is untouched: %+v", arch)
	}
	run := e.runs.last()
	if run.Reason != "looks_wrong" || run.CreatedBy != "user-1" || run.Kind != ReviewRunDraw {
		t.Errorf("the run says why: %+v", run)
	}
}

func TestRegenerate_ARepeatedClientRequestRunsOnce(t *testing.T) {
	e := newRegenEnv(t)
	in := e.req2(domain.RegenArtifact, domain.SectionFlow)
	in.ClientRequestID = "click-1"
	first, err := e.regen.Regenerate(e.ctx, in)
	if err != nil || first.Replayed {
		t.Fatalf("first: %+v %v", first, err)
	}
	calls := e.drawAgent.count()
	again, err := e.regen.Regenerate(e.ctx, in)
	if err != nil || !again.Replayed || again.DrawRun == nil || again.DrawRun.ID != first.DrawRun.ID || e.drawAgent.count() != calls {
		t.Errorf("replay: %+v %v calls=%d", again, err, e.drawAgent.count())
	}
}

func TestRegenerate_LimitsAreEnforcedOnTheServer(t *testing.T) {
	e := newRegenEnv(t)
	e.regen.Config.RegenPerDay = 2
	for i := 0; i < 2; i++ {
		if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); err != nil {
			t.Fatalf("regeneration %d: %v", i, err)
		}
	}
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); !codeIs(err, domain.CodeReviewBudgetExceeded) {
		t.Errorf("over the day's limit: %v", err)
	}
	// The limit is per Request per day: a day later it opens again.
	e.regen.Now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); err != nil {
		t.Errorf("a day later: %v", err)
	}

	f := newRegenEnv(t)
	f.draw.Spawn = func(func()) {}
	f.runner.Spawn = func(func()) {}
	if _, err := f.regen.Regenerate(f.ctx, f.req2(domain.RegenArtifact, domain.SectionFlow)); err != nil {
		t.Fatal(err)
	}
	// A second kind of run starts (the context), then the tenant has two going and the third is refused.
	if _, err := f.regen.Regenerate(f.ctx, f.req2(domain.RegenContext)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.regen.Regenerate(f.ctx, f.req2(domain.RegenArtifact, domain.SectionArchitecture)); !codeIs(err, domain.CodeReviewBudgetExceeded) {
		t.Errorf("two runs at once for the tenant: %v", err)
	}
}

func TestRegenerate_AStaleVersionAndAnActiveRunAndBadInput(t *testing.T) {
	e := newRegenEnv(t)
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	in := e.req2(domain.RegenArtifact, domain.SectionFlow)
	in.ExpectedVersion = view.Stored.Version + 3
	if _, err := e.regen.Regenerate(e.ctx, in); !codeIs(err, domain.CodeReviewStaleRevision) {
		t.Errorf("stale version: %v", err)
	}
	in.ExpectedVersion = view.Stored.Version
	e.draw.Spawn = func(func()) {}
	if _, err := e.regen.Regenerate(e.ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); !codeIs(err, domain.CodeReviewRunInProgress) {
		t.Errorf("a draw is already going: %v", err)
	}
	bad := e.req2(domain.RegenArtifact)
	if _, err := e.regen.Regenerate(e.ctx, bad); !codeIs(err, domain.CodeReviewDecisionInvalid) {
		t.Errorf("an artifact regeneration without a section: %v", err)
	}
	if _, err := e.regen.Regenerate(context.Background(), e.req2(domain.RegenContext)); err == nil {
		t.Error("a call without a tenant must fail")
	}
}

func TestRegenerate_AFailedDrawingNeverCostsTheOneInForce(t *testing.T) {
	e := newRegenEnv(t)
	before, _, _ := e.artifacts.Active(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	e.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(func(h string, m domain.DiagramModel) string {
		return strings.Replace(h, "</svg>", `<script>alert(1)</script></svg>`, 1)
	})}
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); err != nil {
		t.Fatal(err)
	}
	after, ok, _ := e.artifacts.Active(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	if !ok || after.ID != before.ID {
		t.Errorf("the drawing in force must stay: %+v", after)
	}
	revs := e.revisions(t, domain.SectionFlow, "Bug status")
	if revs[0].State != domain.ArtifactFailed {
		t.Errorf("the attempt is kept as failed: %+v", revs[0])
	}
}

func TestRegenerate_ContextScopeRedrawsOnlyTheDiagramsWhoseModelChanged(t *testing.T) {
	e := newRegenEnv(t)
	changed := goodDesignResult()
	changed.Design.Flows[0].Messages = append(changed.Design.Flows[0].Messages, domain.DesignMessage{From: "SVC-05", To: "CT-02", Protocol: "http", Label: "reply", Order: 3})
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(changed)}
	callsBefore := e.drawAgent.count()
	res, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenContext))
	if err != nil || res.ContextRun == nil {
		t.Fatalf("context regeneration: %+v %v", res, err)
	}
	if e.drawAgent.count() != callsBefore+1 {
		t.Fatalf("one draw run for what changed: %d", e.drawAgent.count()-callsBefore)
	}
	prompt := e.drawAgent.calls[callsBefore].Prompt
	if strings.Count(prompt, "<diagram ") != 1 || !strings.Contains(prompt, `section="flow"`) {
		t.Errorf("only the flow model changed, so only the flow is drawn:\n%s", prompt)
	}
	if got := e.runs.last(); got.Kind != ReviewRunDraw {
		t.Errorf("last run: %+v", got)
	}
}

func TestRegenerate_AllScopeMakesEveryDrawingAgain(t *testing.T) {
	e := newRegenEnv(t)
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	callsBefore := e.drawAgent.count()
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenAll)); err != nil {
		t.Fatal(err)
	}
	if e.drawAgent.count() != callsBefore+1 || strings.Count(e.drawAgent.calls[callsBefore].Prompt, "<diagram ") != 2 {
		t.Fatalf("both diagrams are drawn again: calls=%d", e.drawAgent.count()-callsBefore)
	}
	for _, r := range [][]domain.ReviewArtifact{e.revisions(t, domain.SectionFlow, "Bug status"), e.revisions(t, domain.SectionArchitecture, "")} {
		if r[0].Revision != 2 || r[0].Origin != domain.ArtifactOriginRegenerated || r[0].State != domain.ArtifactActive {
			t.Errorf("a new revision even though the model is the same: %+v", r[0])
		}
	}
}

func TestRegenerate_EstimateSpendsNothingAndInventsNoNumber(t *testing.T) {
	e := newRegenEnv(t)
	callsAgent, callsDraw := e.agent.count(), e.drawAgent.count()
	est, err := e.regen.Estimate(e.ctx, e.req.ID, domain.RegenRequest{Scope: domain.RegenAll, Reason: domain.RegenOther})
	if err != nil {
		t.Fatal(err)
	}
	if est.AgentRuns != 2 || est.DiagramsToDraw != 2 || !est.CostKnown || est.ConfirmRequired {
		t.Errorf("all: %+v", est)
	}
	if est.LastContextCost != "0.4200 USD" || est.LastDrawCost != "0.1000 USD" || est.CostUSD < 0.51 || est.CostUSD > 0.53 {
		t.Errorf("the figures are the ones earlier runs reported: %+v", est)
	}
	ctxOnly, _ := e.regen.Estimate(e.ctx, e.req.ID, domain.RegenRequest{Scope: domain.RegenContext, Reason: domain.RegenOther})
	if ctxOnly.AgentRuns != 1 || !ctxOnly.DrawingsDependOnResult {
		t.Errorf("context: %+v", ctxOnly)
	}
	art, _ := e.regen.Estimate(e.ctx, e.req.ID, domain.RegenRequest{Scope: domain.RegenArtifact, Sections: []domain.ReviewSection{domain.SectionFlow}, Reason: domain.RegenOther})
	if art.AgentRuns != 1 || art.DiagramsToDraw != 1 {
		t.Errorf("artifact: %+v", art)
	}
	if e.agent.count() != callsAgent || e.drawAgent.count() != callsDraw {
		t.Error("an estimate never calls an agent")
	}

	// With no earlier run to learn from there is no amount, and so nothing to confirm.
	n := newRunEnv(t, domain.RequestStatusExecuting)
	n.caps.profile = drawProfile(t)
	n.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){func(AgentPromptInput) (AgentPromptResult, error) {
		raw, _ := json.Marshal(goodDesignResult())
		return AgentPromptResult{ResultBlock: raw, AppliedAccessMode: "readonly"}, nil // an agent that reports no usage
	}}
	if _, err := n.runner.Start(n.ctx, StartContextInput{RequestID: n.req.ID}); err != nil {
		t.Fatal(err)
	}
	reg := &ReviewRegenerator{Service: n.svc, Runs: n.runs, Config: domain.OutcomeReviewConfig{ConfirmCostUSD: 0.01}}
	est, err = reg.Estimate(n.ctx, n.req.ID, domain.RegenRequest{Scope: domain.RegenContext, Reason: domain.RegenOther})
	if err != nil || est.CostKnown || est.ConfirmRequired {
		t.Errorf("no figure, no confirmation: %+v %v", est, err)
	}
}

func TestRegenerate_AnExpensiveRunNeedsAYes(t *testing.T) {
	e := newRegenEnv(t)
	e.regen.Config.ConfirmCostUSD = 0.3
	in := e.req2(domain.RegenAll)
	res, err := e.regen.Regenerate(e.ctx, in)
	if !codeIs(err, "REQUEST_REVIEW_CONFIRM_COST_REQUIRED") || !res.Estimate.ConfirmRequired {
		t.Fatalf("expected a confirmation: %+v %v", res, err)
	}
	if e.runs.last().Kind == ReviewRunContext && e.runs.last().Reason != "" {
		t.Error("nothing starts before the yes")
	}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	in.ConfirmCost = true
	if res, err = e.regen.Regenerate(e.ctx, in); err != nil || res.ContextRun == nil {
		t.Errorf("confirmed: %+v %v", res, err)
	}
}

func TestRestore_PutsAnOlderDrawingBackForNothingAndSaysWhatDiffers(t *testing.T) {
	e := newRegenEnv(t)
	oldActive, _, _ := e.artifacts.Active(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	if _, err := e.regen.Regenerate(e.ctx, e.req2(domain.RegenArtifact, domain.SectionFlow)); err != nil {
		t.Fatal(err)
	}
	calls := e.drawAgent.count()
	newActive, _, _ := e.artifacts.Active(e.ctx, e.req.ID, domain.SectionFlow, domain.DiagramVariantDesign, "Bug status")
	if newActive.ID == oldActive.ID {
		t.Fatal("setup: the regeneration made a new revision")
	}
	got, err := e.regen.Restore(e.ctx, e.req.ID, oldActive.ID, "the first one was better", "user-1")
	if err != nil || got.Artifact.ID != oldActive.ID || got.Artifact.State != domain.ArtifactActive {
		t.Fatalf("restore: %+v %v", got, err)
	}
	if e.drawAgent.count() != calls || e.agent.count() != 1 {
		t.Error("a restore calls no agent")
	}
	if got.Diff == nil || !got.Diff.Empty() { // the two revisions were drawn from the same model
		t.Errorf("same model, no difference: %+v", got.Diff)
	}
	active, _ := e.artifacts.ListActive(e.ctx, e.req.ID)
	flows := 0
	for _, a := range active {
		if a.Key.Section == domain.SectionFlow {
			flows++
		}
	}
	if flows != 1 {
		t.Errorf("one active drawing for the flow: %d", flows)
	}
	if _, err := e.regen.Restore(e.ctx, e.req.ID, "00000000-0000-4000-8000-000000000000", "", "user-1"); !codeIs(err, domain.CodeReviewArtifactNotFound) {
		t.Errorf("restore of nothing: %v", err)
	}
	if _, err := e.regen.Restore(e.ctx, e.req.ID, oldActive.ID, strings.Repeat("n", 201), "user-1"); !codeIs(err, domain.CodeReviewDecisionInvalid) {
		t.Errorf("a long note: %v", err)
	}

	// Compare two drawings: the model diff is computed, and only drawings of one diagram compare.
	diff, err := e.regen.Compare(e.ctx, e.req.ID, oldActive.ID, newActive.ID)
	if err != nil || !diff.Empty() {
		t.Errorf("compare: %+v %v", diff, err)
	}
	arch, _, _ := e.artifacts.Active(e.ctx, e.req.ID, domain.SectionArchitecture, domain.DiagramVariantDesign, "")
	if _, err := e.regen.Compare(e.ctx, e.req.ID, oldActive.ID, arch.ID); !codeIs(err, domain.CodeReviewDecisionInvalid) {
		t.Errorf("two different diagrams: %v", err)
	}
}
