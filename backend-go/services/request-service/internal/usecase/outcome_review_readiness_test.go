package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestReadinessGate_ReviewGapsAreReportedApartAndNeverBlockAnEngine(t *testing.T) {
	// An agent that can run the openspec engine but not the review: no read-only enforcement, no tdd-knowledge skill.
	profile := fullProfile()
	profile.Skills["tdd-knowledge"] = domain.SkillStatus{Installed: false}
	g, _ := readyGate(&profileCaps{profile: profile}, &fakeFileStat{info: PathInfo{Exists: true}})

	rep, err := g.CheckForFreshWithReview(context.Background(), "p", nil, true, true)
	if err != nil || !rep.OK || len(rep.Failures) != 0 {
		t.Fatalf("a review that cannot run must not stop the engine: %+v %v", rep, err)
	}
	got := codes(rep.ReviewGaps)
	if len(got) != 2 || got[0] != domain.CodeReviewAgentTooOld || got[1] != domain.CodeReviewSkillMissing {
		t.Fatalf("review gaps = %v", got)
	}
	if rep.ReviewGaps[1].FixHint == "" || rep.ReviewGaps[1].Message == "" {
		t.Errorf("a gap says what is missing and how to fix it: %+v", rep.ReviewGaps[1])
	}

	// With the review off the same dev server shows no review gaps.
	off, _ := g.CheckForFresh(context.Background(), "p", nil, true)
	if len(off.ReviewGaps) != 0 {
		t.Errorf("review gaps appear only when the review is on: %+v", off.ReviewGaps)
	}

	// A fully equipped dev server (read-only enforced, skill installed) has none.
	ready := fullProfile()
	ready.Features = append(append([]string(nil), ready.Features...), domain.FeatureReadonly)
	g2, _ := readyGate(&profileCaps{profile: ready}, &fakeFileStat{info: PathInfo{Exists: true}})
	rep2, _ := g2.CheckForFreshWithReview(context.Background(), "p", nil, false, true)
	if len(rep2.ReviewGaps) != 0 || !rep2.OK {
		t.Errorf("ready: %+v", rep2)
	}
}

func TestReadinessGate_ReviewVerdictIsNotCachedWhenItHasGaps(t *testing.T) {
	profile := fullProfile() // no read-only feature: the review has a gap
	caps := &profileCaps{profile: profile}
	g, _ := readyGate(caps, &fakeFileStat{info: PathInfo{Exists: true}})
	for i := 0; i < 2; i++ {
		rep, _ := g.CheckFor(context.Background(), "p", nil, true) // the cached path must not hide a gap from a later review check
		_ = rep
	}
	rep, _ := g.CheckForFreshWithReview(context.Background(), "p", nil, true, true)
	if len(rep.ReviewGaps) == 0 {
		t.Error("an engine check that was clean and cached must not hide the review gap")
	}
}

func TestReadinessGate_UnreadableCapabilitiesSayTheReviewCannotBeConfirmed(t *testing.T) {
	g, _ := readyGate(&profileCaps{err: errors.New("offline")}, &fakeFileStat{info: PathInfo{Exists: true}})
	rep, _ := g.CheckForFreshWithReview(context.Background(), "p", nil, false, true)
	if got := codes(rep.ReviewGaps); len(got) != 1 || got[0] != "REQUEST_REVIEW_AGENT_UNKNOWN" {
		t.Errorf("review gaps = %v", got)
	}
}

func TestDesignLock_ReusedUntilItExpiresOrTheWordingChanges(t *testing.T) {
	store := newMemContexts()
	now := time.Now().UTC()
	lock := ReviewDesignLock{Store: store, TTL: time.Hour, Now: func() time.Time { return now }}
	design := domain.DesignModel{Available: true, Snapshot: "snap-1", DocSha256: map[string]string{"docs/SAD.md": runSADSum},
		Containers: []domain.DesignContainer{{ID: "CT-02", Label: "ms-gateway", Kind: "service"}}}
	saved, err := lock.Save(context.Background(), "req-1", design)
	if err != nil || saved.Digest == "" {
		t.Fatal(saved, err)
	}
	got, ok, err := lock.Load(context.Background(), "req-1")
	if err != nil || !ok || got.Digest != saved.Digest || len(got.Design.Containers) != 1 {
		t.Fatalf("load: %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := lock.Load(context.Background(), "req-2"); ok {
		t.Error("a lock belongs to one Request")
	}
	// The same snapshot and documents give the same digest; another snapshot gives another.
	again, _ := lock.Save(context.Background(), "req-1", design)
	design.Snapshot = "snap-2"
	other, _ := lock.Save(context.Background(), "req-1", design)
	if again.Digest != saved.Digest || other.Digest == saved.Digest {
		t.Errorf("digests: %s %s %s", saved.Digest, again.Digest, other.Digest)
	}
	// After the TTL the lock is gone.
	lock.Now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, ok, _ := lock.Load(context.Background(), "req-1"); ok {
		t.Error("an expired lock must not be used")
	}
	// A lock made with other wording is not used, and neither is one that cannot be read.
	lock.Now = func() time.Time { return now }
	store.Put(context.Background(), ReviewContextRecord{ID: "x", RequestID: "req-3", Kind: ReviewContextDesign, Digest: "d",
		Payload: []byte(`{"promptVersion":"outcome_review_context/0","schema":1,"design":{"available":true}}`), ExpiresAt: now.Add(time.Hour)})
	if _, ok, _ := lock.Load(context.Background(), "req-3"); ok {
		t.Error("a lock made with another prompt version must not be used")
	}
	store.Put(context.Background(), ReviewContextRecord{ID: "y", RequestID: "req-4", Kind: ReviewContextDesign, Digest: "d", Payload: []byte(`not json`), ExpiresAt: now.Add(time.Hour)})
	if _, ok, _ := lock.Load(context.Background(), "req-4"); ok {
		t.Error("an unreadable lock must not be used")
	}
}

func TestContextRun_ACachedResultExpiresWithItsTTL(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	now := time.Now().UTC()
	e.runner.Now = func() time.Time { return now }
	e.runner.Config.ContextTTL = time.Hour
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	e.runner.Now = func() time.Time { return now.Add(30 * time.Minute) }
	if got, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil || !got.ContextCached || e.agent.count() != 1 {
		t.Fatalf("inside the TTL: cached=%v calls=%d err=%v", got.ContextCached, e.agent.count(), err)
	}
	e.runner.Now = func() time.Time { return now.Add(2 * time.Hour) }
	got, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID})
	if err != nil || got.ContextCached || got.Run == nil || e.agent.count() != 2 {
		t.Fatalf("after the TTL the agent is asked again: cached=%v calls=%d err=%v", got.ContextCached, e.agent.count(), err)
	}
}

func TestContextRun_EstimateReportsCacheLockAndGapsWithoutSpending(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	est, _ := e.svc.Estimate(e.ctx, e.req.ID)
	ce, err := e.runner.EstimateContext(e.ctx, e.req.ID, est)
	if err != nil || ce.Cached || ce.DesignLocked || !ce.Runnable || len(ce.Gaps) != 0 {
		t.Fatalf("before any run: %+v err=%v", ce, err)
	}
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	if _, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID}); err != nil {
		t.Fatal(err)
	}
	est, _ = e.svc.Estimate(e.ctx, e.req.ID)
	if ce, _ = e.runner.EstimateContext(e.ctx, e.req.ID, est); !ce.Cached {
		t.Errorf("after a run the result is kept: %+v", ce)
	}
	callsBefore := e.agent.count()

	e.caps.profile = capabilityProfile(t, []string{domain.FeatureReadonly}, `[{"name":"tdd-knowledge","installed":false}]`)
	e.files.exists = map[string]bool{}
	ce, _ = e.runner.EstimateContext(e.ctx, e.req.ID, est)
	want := map[string]bool{"kg_unavailable": true, domain.CodeReviewAgentTooOld: true, domain.CodeReviewSkillMissing: true}
	if ce.Runnable || len(ce.Gaps) != 3 {
		t.Errorf("gaps = %v", ce.Gaps)
	}
	for _, g := range ce.Gaps {
		if !want[g] {
			t.Errorf("unexpected gap %s", g)
		}
	}
	e.runner.Conns = fixedConns{err: ErrNoDevServer}
	if ce, _ = e.runner.EstimateContext(e.ctx, e.req.ID, est); ce.Runnable || len(ce.Gaps) != 1 || ce.Gaps[0] != "no_dev_server" {
		t.Errorf("no dev server: %+v", ce)
	}
	if e.agent.count() != callsBefore {
		t.Error("an estimate never calls the agent")
	}
}

func TestContextRun_ListRunsBelongToTheRequestsReview(t *testing.T) {
	e := newRunEnv(t, domain.RequestStatusExecuting)
	e.agent.replies = []func(AgentPromptInput) (AgentPromptResult, error){agentReply(goodDesignResult())}
	first, err := e.runner.Start(e.ctx, StartContextInput{RequestID: e.req.ID})
	if err != nil {
		t.Fatal(err)
	}
	runs, err := e.runner.ListRuns(e.ctx, e.req.ID, "")
	if err != nil || len(runs) != 1 || runs[0].ID != first.Run.ID || runs[0].Status != ReviewRunSucceeded {
		t.Fatalf("runs = %+v err=%v", runs, err)
	}
	if byID, err := e.runner.ListRuns(e.ctx, e.req.ID, first.View.Stored.ID); err != nil || len(byID) != 1 {
		t.Errorf("by review id: %+v %v", byID, err)
	}
	if _, err := e.runner.ListRuns(e.ctx, "another-request", first.View.Stored.ID); err == nil {
		t.Error("a review is not listed through another Request")
	}
}
