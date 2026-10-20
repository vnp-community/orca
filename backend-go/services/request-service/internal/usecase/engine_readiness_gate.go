package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type PreflightFailure struct {
	Code    string
	Message string
	FixHint string
}

// PreflightReport separates what stops an engine (Failures) from what only means the knowledge hub will not be used (Gaps):
// without the hub a Solution and a Plan are still written, so a gap never turns an engine off.
type PreflightReport struct {
	OK       bool
	Failures []PreflightFailure
	Gaps     []PreflightFailure
	// ReviewGaps say why the outcome review cannot run its context run here; like Gaps they never turn an engine off.
	ReviewGaps      []PreflightFailure
	Warnings        []string
	CheckedAt       time.Time
	OpenSpecVersion string
}

// openSpecProbeTimeoutMs bounds `openspec --version`; a healthy install answers in well under a second.
const openSpecProbeTimeoutMs = 10_000

// readinessCacheTTL is how long a clean verdict is reused; a failing or gapped one is never kept, so a fix shows at once.
const readinessCacheTTL = 10 * time.Minute

const (
	knowledgeConfigPath = ".tdd-agent/config.toml"
	tddAgentToolID      = "tdd-agent"
)

// DefaultSkillsRepositoryURL is where `npx skills add` finds the knowledge-hub skills (the repository's skills/ folder).
const DefaultSkillsRepositoryURL = "https://github.com/vnp-community/orca"

// RepoFileStat says whether a path exists in a worktree; the file system answers, the knowledge graph is never asked.
type RepoFileStat interface {
	Stat(ctx context.Context, ws Workspace, relPath string) (PathInfo, error)
}

// EngineReadinessGate says whether a project's dev server can run the openspec engine and reach the knowledge hub. It asks
// the dev server itself, so a missing or old openspec shows up here and not as a failed run later.
type EngineReadinessGate struct {
	conns      AnalysisConnectionResolver
	exec       DevServerExecutor
	minVersion string
	now        func() time.Time
	caps       DevServerCapabilityReader
	files      RepoFileStat
	repo       RepoStateProbe
	skillsRepo string
	observer   EngineObserver

	mu    sync.Mutex
	clean map[string]time.Time
}

func NewEngineReadinessGate(conns AnalysisConnectionResolver, exec DevServerExecutor, minVersion string) *EngineReadinessGate {
	return &EngineReadinessGate{
		conns: conns, exec: exec, minVersion: minVersion, now: func() time.Time { return time.Now().UTC() },
		skillsRepo: DefaultSkillsRepositoryURL, clean: map[string]time.Time{},
	}
}

// WithCapabilities adds the agent's feature, tool and skill report to the check. Without it only openspec is probed.
func (g *EngineReadinessGate) WithCapabilities(c DevServerCapabilityReader) *EngineReadinessGate {
	g.caps = c
	return g
}

// WithRepoFiles lets the check confirm that the repository has .tdd-agent/config.toml.
func (g *EngineReadinessGate) WithRepoFiles(f RepoFileStat) *EngineReadinessGate {
	g.files = f
	return g
}

// WithRepoState lets the check warn about uncommitted changes in the project's repository.
func (g *EngineReadinessGate) WithRepoState(p RepoStateProbe) *EngineReadinessGate {
	g.repo = p
	return g
}

// WithObserver counts the blockers each fresh check finds, for the engine metrics.
func (g *EngineReadinessGate) WithObserver(o EngineObserver) *EngineReadinessGate {
	g.observer = o
	return g
}

func (g *EngineReadinessGate) WithSkillsRepository(url string) *EngineReadinessGate {
	if url != "" {
		g.skillsRepo = url
	}
	return g
}

// Check is what a run needs: openspec must work. It never returns an error for a failed probe: the verdict is in the report.
// An error means the check itself could not run (no tenant, the resolver broke).
func (g *EngineReadinessGate) Check(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	return g.CheckFor(ctx, projectID, settings, true)
}

// CheckFor is Check for a switch being turned on: with openspec=false only the knowledge hub is looked at (a project that
// turns on solution_agent alone has no need of openspec).
func (g *EngineReadinessGate) CheckFor(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings, openspec bool) (PreflightReport, error) {
	return g.checkFor(ctx, projectID, settings, gateScope{openspec: openspec}, false)
}

// CheckForFresh is CheckFor for a person asking "is it ready now": it ignores the cached clean verdict and asks the dev server
// agent for its capabilities again, which its profile cache would otherwise keep for a day (a skill installed a minute ago
// would still show as missing).
func (g *EngineReadinessGate) CheckForFresh(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings, openspec bool) (PreflightReport, error) {
	return g.checkFor(ctx, projectID, settings, gateScope{openspec: openspec}, true)
}

// CheckForFreshWithReview is CheckForFresh that also says whether the outcome review can run on the dev server.
func (g *EngineReadinessGate) CheckForFreshWithReview(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings, openspec, review bool) (PreflightReport, error) {
	return g.checkFor(ctx, projectID, settings, gateScope{openspec: openspec, review: review}, true)
}

// gateScope names what a check looks at; the engine and the review each add their own requirements.
type gateScope struct{ openspec, review bool }

func (g *EngineReadinessGate) checkFor(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings, scope gateScope, fresh bool) (PreflightReport, error) {
	report, err := g.check(ctx, projectID, settings, scope, fresh)
	if err == nil {
		// A clean verdict taken from the cache has no failures, so only a fresh finding is counted.
		for _, f := range report.Failures {
			observerOrNoop(g.observer).PreflightFailed(f.Code)
		}
	}
	return report, err
}

func (g *EngineReadinessGate) check(ctx context.Context, projectID string, settings *domain.ProjectEngineSettings, scope gateScope, fresh bool) (PreflightReport, error) {
	openspec := scope.openspec
	report := PreflightReport{OK: true, CheckedAt: g.now()}
	conn, err := g.conns.ResolveForProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return failed(report, "REQUEST_ENGINE_NO_CONNECTION", "no dev server can run this project's repo", "Connect a dev server and add the repo to the project."), nil
		}
		return report, err
	}
	if conn.RepoPath == "" {
		return failed(report, "REQUEST_ENGINE_REPO_NOT_RESOLVED", "the repo has no folder path on the dev server", "Add the repo with a path the dev server can open."), nil
	}

	want := g.minVersion
	if settings != nil && settings.MinVersion != "" {
		want = settings.MinVersion
	}
	key := fmt.Sprintf("%s|%s|%s|%t|%t|%s", conn.DevServerID, conn.ConnectionID, conn.RepoPath, openspec, scope.review, want)
	if !fresh && g.cachedClean(key) {
		return report, nil
	}

	if openspec {
		if err := g.checkOpenSpec(ctx, conn, want, &report); err != nil {
			return report, err
		}
	}
	g.checkAgent(ctx, conn, scope, fresh, &report)
	g.checkRepoFiles(ctx, conn, &report)
	if len(report.Failures) == 0 && len(report.Gaps) == 0 && len(report.ReviewGaps) == 0 {
		g.remember(key)
	}
	return report, nil
}

func (g *EngineReadinessGate) cachedClean(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	at, ok := g.clean[key]
	if !ok {
		return false
	}
	if g.now().Sub(at) >= readinessCacheTTL {
		delete(g.clean, key)
		return false
	}
	return true
}

func (g *EngineReadinessGate) remember(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clean[key] = g.now()
}

func (g *EngineReadinessGate) checkOpenSpec(ctx context.Context, conn AnalysisConnection, want string, report *PreflightReport) error {
	res, err := g.exec.Exec(ctx, conn, ExecInput{Binary: "openspec", Args: []string{"--version"}, Cwd: conn.RepoPath, TimeoutMs: openSpecProbeTimeoutMs})
	switch {
	case err != nil:
		return err
	case res.TimedOut:
		block(report, "REQUEST_ENGINE_OPENSPEC_TIMEOUT", "openspec did not answer in time", "Run `openspec --version` on the dev server.")
		return nil
	case res.SpawnError != "" || res.ExitCode != 0:
		block(report, "REQUEST_ENGINE_OPENSPEC_MISSING", "openspec is not installed or does not start on the dev server", "Install the OpenSpec CLI on the dev server.")
		return nil
	}

	report.OpenSpecVersion = strings.TrimSpace(res.Stdout)
	ver, err := domain.ParseSemver(res.Stdout)
	if err != nil {
		block(report, "REQUEST_ENGINE_OPENSPEC_VERSION", "could not read the openspec version from "+quote(report.OpenSpecVersion), "Update the OpenSpec CLI.")
		return nil
	}
	if want == "" {
		return nil
	}
	min, err := domain.ParseSemver(want)
	if err != nil {
		report.Warnings = append(report.Warnings, "the configured minimum openspec version "+quote(want)+" is not a version; it was ignored")
		return nil
	}
	if domain.CompareSemver(ver, min) < 0 {
		block(report, "REQUEST_ENGINE_OPENSPEC_VERSION", "openspec "+report.OpenSpecVersion+" is older than the required "+want, "Update the OpenSpec CLI to "+want+" or newer.")
	}
	return nil
}

// checkAgent reads the agent's own report: what the openspec engine needs is a blocker, what the knowledge hub needs is a gap.
func (g *EngineReadinessGate) checkAgent(ctx context.Context, conn AnalysisConnection, scope gateScope, fresh bool, report *PreflightReport) {
	openspec := scope.openspec
	if g.caps == nil {
		return
	}
	profile, err := g.caps.Get(ctx, DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}, fresh)
	if err != nil {
		if scope.review {
			reviewGap(report, "REQUEST_REVIEW_AGENT_UNKNOWN", "the dev server agent's capabilities could not be read, so the outcome review cannot be confirmed", "Check that the dev server is connected, then try again.")
		}
		if openspec {
			block(report, "REQUEST_ENGINE_AGENT_UNKNOWN", "the dev server agent's capabilities could not be read", "Check that the dev server is connected, then try again.")
		}
		gap(report, "REQUEST_KNOWLEDGE_AGENT_UNKNOWN", "the dev server agent's capabilities could not be read, so the knowledge hub cannot be confirmed", "Check that the dev server is connected, then try again.")
		return
	}
	if scope.review {
		for _, rg := range domain.ReviewContextGaps(profile) {
			hint := "Update the Orca agent on the dev server."
			if rg.Code == domain.CodeReviewSkillMissing {
				hint = SkillInstallCommand(g.skillsRepo)
			}
			reviewGap(report, rg.Code, rg.Message, hint)
		}
	}
	if openspec && !domain.SupportsOpenSpecAuthoring(profile) {
		block(report, "REQUEST_ENGINE_AGENT_TOO_OLD", "the dev server agent cannot author a change in a worktree under a permission profile", "Update the Orca agent on the dev server.")
	}
	if !domain.SupportsKnowledgeHub(profile) {
		gap(report, "REQUEST_KNOWLEDGE_AGENT_TOO_OLD", "the dev server agent cannot load the knowledge hub", "Update the Orca agent on the dev server.")
		return
	}
	if t := profile.Tool(tddAgentToolID); t.Known && !t.Installed {
		gap(report, "REQUEST_KNOWLEDGE_TDD_AGENT_MISSING", "tdd-agent is not on the dev server's PATH", "Install tdd-agent on the dev server and make sure `tdd-agent` runs in a shell.")
	}
	if profile.SkillsKnown {
		var missing []string
		for _, n := range domain.KnowledgeSkillNames {
			if st, ok := profile.Skills[n]; !ok || !st.Installed {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			gap(report, "REQUEST_KNOWLEDGE_SKILLS_MISSING", "skills not installed on the dev server: "+strings.Join(missing, ", "), SkillInstallCommand(g.skillsRepo))
		}
	}
}

// SkillInstallCommand is the command that installs the knowledge-hub skills on a dev server.
func SkillInstallCommand(repoURL string) string {
	return "Run: npx skills add " + repoURL + " --skill " + strings.Join(domain.KnowledgeSkillNames, " ") + " --global"
}

// checkRepoFiles confirms the repository carries .tdd-agent/config.toml (the file system, not the graph) and warns about
// uncommitted changes. Neither can stop an engine.
func (g *EngineReadinessGate) checkRepoFiles(ctx context.Context, conn AnalysisConnection, report *PreflightReport) {
	if conn.WorktreeID == "" {
		return
	}
	if g.files != nil {
		info, err := g.files.Stat(ctx, Workspace{WorktreeID: conn.WorktreeID, Path: conn.RepoPath}, knowledgeConfigPath)
		switch {
		case err != nil:
			report.Warnings = append(report.Warnings, "could not look for "+knowledgeConfigPath+" in the repository")
		case !info.Exists || info.IsDir:
			gap(report, "REQUEST_KNOWLEDGE_NOT_INITIALIZED", "the repository has no "+knowledgeConfigPath, "Set tdd-agent up in the repository (it writes "+knowledgeConfigPath+") and commit or keep it on the dev server.")
		}
	}
	if g.repo != nil {
		if snap, err := g.repo.Snapshot(ctx, conn.WorktreeID); err == nil && len(snap.Files) > 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("the project's repository has %d uncommitted file(s); proposals use their own worktree and are not affected", len(snap.Files)))
		}
	}
}

func failed(r PreflightReport, code, msg, hint string) PreflightReport {
	block(&r, code, msg, hint)
	return r
}

func block(r *PreflightReport, code, msg, hint string) {
	r.OK = false
	r.Failures = append(r.Failures, PreflightFailure{Code: code, Message: msg, FixHint: hint})
}

func reviewGap(r *PreflightReport, code, msg, hint string) {
	r.ReviewGaps = append(r.ReviewGaps, PreflightFailure{Code: code, Message: msg, FixHint: hint})
}

func gap(r *PreflightReport, code, msg, hint string) {
	r.Gaps = append(r.Gaps, PreflightFailure{Code: code, Message: msg, FixHint: hint})
}

func quote(s string) string { return `"` + s + `"` }
