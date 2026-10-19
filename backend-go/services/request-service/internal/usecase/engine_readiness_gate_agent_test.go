package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type profileCaps struct {
	profile    domain.DevServerCapability
	err        error
	calls      int
	refreshing []bool
}

func (p *profileCaps) Get(_ context.Context, _ DevServerRef, refresh bool) (domain.DevServerCapability, error) {
	p.calls++
	p.refreshing = append(p.refreshing, refresh)
	return p.profile, p.err
}

type fakeFileStat struct {
	info PathInfo
	err  error
	seen []string
}

func (f *fakeFileStat) Stat(_ context.Context, ws Workspace, rel string) (PathInfo, error) {
	f.seen = append(f.seen, ws.WorktreeID+":"+rel)
	return f.info, f.err
}

var openspecAgentFeatures = []string{
	"agent.execPrompt", "agent.execPrompt.workspaceKind", "agent.execPrompt.changes", "agent.execPrompt.resultBlock",
	domain.FeaturePermissionProfile, domain.FeatureKnowledgeHub,
}

func fullProfile() domain.DevServerCapability {
	skills := map[string]domain.SkillStatus{}
	for _, n := range domain.KnowledgeSkillNames {
		skills[n] = domain.SkillStatus{Installed: true, SHA256: "ab"}
	}
	return domain.DevServerCapability{
		Features: openspecAgentFeatures, SkillsKnown: true, Skills: skills,
		Tools: map[string]domain.ToolStatus{"tdd-agent": {Known: true, Installed: true}},
	}
}

var worktreeConn = fakeConns{conn: AnalysisConnection{ConnectionID: "c", DevServerID: "ds", RepoPath: "/srv/repo", WorktreeID: "wt"}}

func readyGate(caps DevServerCapabilityReader, files RepoFileStat) (*EngineReadinessGate, *fakeExec) {
	ex := &fakeExec{res: ExecResult{Stdout: "1.3.0"}}
	return NewEngineReadinessGate(worktreeConn, ex, "").WithCapabilities(caps).WithRepoFiles(files), ex
}

func codes(notes []PreflightFailure) []string {
	var out []string
	for _, n := range notes {
		out = append(out, n.Code)
	}
	return out
}

func TestReadinessGate_AFullyEquippedDevServerIsReadyWithNoGaps(t *testing.T) {
	files := &fakeFileStat{info: PathInfo{Exists: true}}
	g, _ := readyGate(&profileCaps{profile: fullProfile()}, files)
	rep, err := g.Check(context.Background(), "p", nil)
	if err != nil || !rep.OK || len(rep.Failures) != 0 || len(rep.Gaps) != 0 {
		t.Fatalf("%+v %v", rep, err)
	}
	if len(files.seen) != 1 || files.seen[0] != "wt:.tdd-agent/config.toml" {
		t.Fatalf("the config file must be looked for in the repo's worktree: %v", files.seen)
	}
}

func TestReadinessGate_EachMissingPieceOfTheKnowledgeHubIsOneGapWithAHintAndNeverABlocker(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.DevServerCapability)
		stat   PathInfo
		want   string
		hint   string
	}{
		{"agent cannot load the hub", func(c *domain.DevServerCapability) { c.Features = openspecAgentFeatures[:5] }, PathInfo{Exists: true}, "REQUEST_KNOWLEDGE_AGENT_TOO_OLD", "Update the Orca agent"},
		{"tdd-agent not installed", func(c *domain.DevServerCapability) {
			c.Tools = map[string]domain.ToolStatus{"tdd-agent": {Known: true}}
		}, PathInfo{Exists: true}, "REQUEST_KNOWLEDGE_TDD_AGENT_MISSING", "Install tdd-agent"},
		{"a skill is missing", func(c *domain.DevServerCapability) { c.Skills["tdd-orca-task"] = domain.SkillStatus{} }, PathInfo{Exists: true}, "REQUEST_KNOWLEDGE_SKILLS_MISSING", "npx skills add"},
		{"no .tdd-agent in the repo", func(*domain.DevServerCapability) {}, PathInfo{}, "REQUEST_KNOWLEDGE_NOT_INITIALIZED", "Set tdd-agent up"},
		{".tdd-agent/config.toml is a directory", func(*domain.DevServerCapability) {}, PathInfo{Exists: true, IsDir: true}, "REQUEST_KNOWLEDGE_NOT_INITIALIZED", "Set tdd-agent up"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fullProfile()
			tc.mutate(&p)
			g, _ := readyGate(&profileCaps{profile: p}, &fakeFileStat{info: tc.stat})
			rep, err := g.Check(context.Background(), "p", nil)
			if err != nil || !rep.OK || len(rep.Failures) != 0 {
				t.Fatalf("a knowledge gap must not stop the engine: %+v %v", rep, err)
			}
			if got := codes(rep.Gaps); len(got) != 1 || got[0] != tc.want || !strings.Contains(rep.Gaps[0].FixHint, tc.hint) {
				t.Fatalf("gaps = %+v, want %s with hint containing %q", rep.Gaps, tc.want, tc.hint)
			}
		})
	}
}

func TestReadinessGate_AnAgentThatCannotAuthorAChangeBlocksTheOpenSpecEngineOnly(t *testing.T) {
	p := fullProfile()
	p.Features = []string{domain.FeatureKnowledgeHub}
	g, ex := readyGate(&profileCaps{profile: p}, &fakeFileStat{info: PathInfo{Exists: true}})
	rep, _ := g.Check(context.Background(), "p", nil)
	if rep.OK || len(rep.Failures) != 1 || rep.Failures[0].Code != "REQUEST_ENGINE_AGENT_TOO_OLD" || len(ex.seen) != 1 {
		t.Fatalf("%+v", rep)
	}
	// Turning on only solution_agent needs neither openspec nor the authoring features.
	ex.seen = nil
	rep, _ = g.CheckFor(context.Background(), "p", nil, false)
	if !rep.OK || len(rep.Failures) != 0 || len(ex.seen) != 0 {
		t.Fatalf("knowledge-only check ran openspec or blocked: %+v seen=%d", rep, len(ex.seen))
	}
}

func TestReadinessGate_UnreadableCapabilitiesBlockOpenSpecAndOnlyWarnTheKnowledgeHub(t *testing.T) {
	g, _ := readyGate(&profileCaps{err: errors.New("relay down")}, &fakeFileStat{info: PathInfo{Exists: true}})
	rep, err := g.Check(context.Background(), "p", nil)
	if err != nil || rep.OK || codes(rep.Failures)[0] != "REQUEST_ENGINE_AGENT_UNKNOWN" || len(rep.Gaps) != 1 {
		t.Fatalf("%+v %v", rep, err)
	}
	rep, _ = g.CheckFor(context.Background(), "p", nil, false)
	if !rep.OK || len(rep.Gaps) != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestReadinessGate_ARepoFileProbeThatBreaksIsAWarningNotAGap(t *testing.T) {
	g, _ := readyGate(&profileCaps{profile: fullProfile()}, &fakeFileStat{err: errors.New("git-gateway down")})
	rep, _ := g.Check(context.Background(), "p", nil)
	if !rep.OK || len(rep.Gaps) != 0 || len(rep.Warnings) != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestReadinessGate_UncommittedFilesOnlyWarn(t *testing.T) {
	probe := &scriptedProbe{snaps: []RepoSnapshot{{Branch: "main", Files: []RepoFileState{{Path: "a", State: "modified"}, {Path: "b", State: "untracked"}}}}}
	g, _ := readyGate(&profileCaps{profile: fullProfile()}, &fakeFileStat{info: PathInfo{Exists: true}})
	rep, _ := g.WithRepoState(probe).Check(context.Background(), "p", nil)
	if !rep.OK || len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], "2 uncommitted") {
		t.Fatalf("%+v", rep)
	}
}

func TestReadinessGate_ACleanVerdictIsReusedForTenMinutesAndAnythingElseIsAskedAgain(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	caps := &profileCaps{profile: fullProfile()}
	g, ex := readyGate(caps, &fakeFileStat{info: PathInfo{Exists: true}})
	g.now = func() time.Time { return now }
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if rep, _ := g.Check(ctx, "p", nil); !rep.OK {
			t.Fatalf("%+v", rep)
		}
	}
	if len(ex.seen) != 1 {
		t.Fatalf("openspec asked %d times within the window, want 1", len(ex.seen))
	}
	now = now.Add(9*time.Minute + 59*time.Second)
	g.Check(ctx, "p", nil)
	if len(ex.seen) != 1 {
		t.Fatal("asked again before ten minutes passed")
	}
	now = now.Add(2 * time.Second)
	g.Check(ctx, "p", nil)
	if len(ex.seen) != 2 {
		t.Fatalf("not asked again after ten minutes: %d", len(ex.seen))
	}
	// A different minimum version is a different question.
	g.Check(ctx, "p", &domain.ProjectEngineSettings{MinVersion: "1.0.0"})
	if len(ex.seen) != 3 {
		t.Fatalf("a changed minimum version reused the cached verdict: %d", len(ex.seen))
	}
}

func TestReadinessGate_AFailureOrAGapIsNeverCachedSoAFixShowsAtOnce(t *testing.T) {
	files := &fakeFileStat{info: PathInfo{}}
	g, ex := readyGate(&profileCaps{profile: fullProfile()}, files)
	ctx := context.Background()
	if rep, _ := g.Check(ctx, "p", nil); len(rep.Gaps) != 1 {
		t.Fatalf("%+v", rep)
	}
	files.info = PathInfo{Exists: true} // the operator fixed it
	if rep, _ := g.Check(ctx, "p", nil); len(rep.Gaps) != 0 || len(ex.seen) != 2 {
		t.Fatalf("a gap was cached: %+v probes=%d", rep, len(ex.seen))
	}
	g2, ex2 := readyGate(&profileCaps{profile: fullProfile()}, &fakeFileStat{info: PathInfo{Exists: true}})
	ex2.res = ExecResult{ExitCode: 127}
	g2.Check(ctx, "p", nil)
	ex2.res = ExecResult{Stdout: "1.3.0"}
	if rep, _ := g2.Check(ctx, "p", nil); !rep.OK {
		t.Fatalf("a failure was cached: %+v", rep)
	}
}

func TestSkillInstallCommand_NamesEveryKnowledgeSkill(t *testing.T) {
	cmd := SkillInstallCommand(DefaultSkillsRepositoryURL)
	for _, n := range domain.KnowledgeSkillNames {
		if !strings.Contains(cmd, n) {
			t.Errorf("install command lacks %s: %s", n, cmd)
		}
	}
	if !strings.HasSuffix(cmd, "--global") || !strings.Contains(cmd, "npx skills add "+DefaultSkillsRepositoryURL) {
		t.Fatal(cmd)
	}
}

func TestReadinessGate_AnExplicitCheckAsksTheAgentAgainAndIgnoresTheCleanCache(t *testing.T) {
	caps := &profileCaps{profile: fullProfile()}
	g, ex := readyGate(caps, &fakeFileStat{info: PathInfo{Exists: true}})
	ctx := context.Background()
	if rep, _ := g.Check(ctx, "p", nil); !rep.OK || len(rep.Gaps) != 0 {
		t.Fatalf("%+v", rep)
	}
	if rep, _ := g.Check(ctx, "p", nil); !rep.OK || caps.calls != 1 || len(ex.seen) != 1 {
		t.Fatalf("a run's check must reuse the clean verdict: calls=%d probes=%d", caps.calls, len(ex.seen))
	}
	if rep, err := g.CheckForFresh(ctx, "p", nil, true); err != nil || !rep.OK || caps.calls != 2 || len(ex.seen) != 2 {
		t.Fatalf("an explicit check must probe again: %+v %v calls=%d probes=%d", rep, err, caps.calls, len(ex.seen))
	}
	if !caps.refreshing[1] || caps.refreshing[0] {
		t.Fatalf("only the explicit check forces the agent to re-read its capabilities: %v", caps.refreshing)
	}
}
