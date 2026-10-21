package usecase

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// The builtin engine draws without an agent, a dev server or a skill, and a second open costs nothing.
func TestDrawRun_BuiltinEngineDrawsEveryModelWithoutAnAgent(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.draw.Config.DrawEngine = domain.DrawEngineBuiltin
	e.caps.profile = domain.DevServerCapability{} // nothing installed anywhere

	first := e.start(t, StartDrawInput{})
	if first.Run != nil || first.Drawn != 2 || first.Failed != 0 || e.drawAgent.count() != 0 {
		t.Fatalf("first: %+v agent calls=%d", first, e.drawAgent.count())
	}
	active, _ := e.artifacts.ListActive(e.ctx, e.req.ID)
	if len(active) != 2 {
		t.Fatalf("active drawings: %d", len(active))
	}
	for _, a := range active {
		if a.Key.SkillVersion != domain.BuiltinDrawSkillVersion || a.ViewBox == "" {
			t.Errorf("drawing: %+v", a)
		}
		content, err := e.store.Read(e.ctx, e.req.ID, a.ID, "")
		if err != nil || !strings.Contains(string(content.Blob.Content), "<svg") {
			t.Fatalf("read: %v", err)
		}
	}
	if second := e.start(t, StartDrawInput{}); second.Cached != 2 || second.Drawn != 0 {
		t.Errorf("second open: %+v", second)
	}
	view, _, _ := e.svc.Latest(e.ctx, e.req.ID)
	refs, err := e.store.Annotate(e.ctx, e.req.ID, view.Review.Diagrams, e.profiles.p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		if r.Status != "cached" {
			t.Errorf("annotated: %+v", r)
		}
	}
}

func TestDrawRun_AutoEngineFallsBackToBuiltinOnlyWhenNoAgentCanDraw(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.draw.Config.DrawEngine = domain.DrawEngineAuto
	e.caps.profile = domain.DevServerCapability{} // the skill is not installed

	res := e.start(t, StartDrawInput{})
	if res.Drawn != 2 || res.Run != nil || e.drawAgent.count() != 0 {
		t.Fatalf("auto without a skill must draw itself: %+v", res)
	}

	e2 := newDrawEnv(t, domain.RequestStatusExecuting)
	e2.draw.Config.DrawEngine = domain.DrawEngineAuto
	e2.drawAgent.replies = []func(AgentPromptInput) (AgentPromptResult, error){drawReply(nil)}
	if res := e2.start(t, StartDrawInput{}); res.Run == nil || res.Drawn != 0 {
		t.Fatalf("auto with the skill must use the agent: %+v", res)
	}
}

func TestDrawRun_AgentEngineStillRefusesWithoutTheSkill(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.draw.Config.DrawEngine = domain.DrawEngineAgent
	e.caps.profile = domain.DevServerCapability{}
	if _, err := e.draw.Start(e.ctx, StartDrawInput{RequestID: e.req.ID}); err == nil || domainCode(err) != domain.CodeReviewSkillMissing {
		t.Fatalf("want %s, got %v", domain.CodeReviewSkillMissing, err)
	}
}

func TestDrawRun_ForceDrawsAgainWithoutChangingTheContentStored(t *testing.T) {
	e := newDrawEnv(t, domain.RequestStatusExecuting)
	e.draw.Config.DrawEngine = domain.DrawEngineBuiltin
	e.start(t, StartDrawInput{})
	res := e.start(t, StartDrawInput{Force: true, Origin: domain.ArtifactOriginRegenerated, Reason: "looks_wrong"})
	if res.Drawn != 2 {
		t.Fatalf("a regeneration draws again: %+v", res)
	}
	active, _ := e.artifacts.ListActive(e.ctx, e.req.ID)
	if len(active) != 2 {
		t.Fatalf("one active drawing per slot, got %d", len(active))
	}
}
