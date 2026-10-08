package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RunAgentReadonlyAnalysis runs a diagnosis, findings or answer through agent.execPrompt on the project's repo.
// Read-only is enforced by the agent when it advertises the feature, and always checked by prompt and repo comparison:
// no layer is absolute, so the run records which ones were active.
type RunAgentReadonlyAnalysis struct {
	requests  RequestRepository
	solutions SolutionStore
	conns     AnalysisConnectionResolver
	agent     AgentPromptRunner
	probe     RepoStateProbe
	caps      DevServerCapabilityReader
	projects  ProjectContextReader
	writer    *AnalysisResultWriter
	settings  AnalysisSettings
}

var _ AnalysisWorker = (*RunAgentReadonlyAnalysis)(nil)

type AgentReadonlyDeps struct {
	Requests  RequestRepository
	Solutions SolutionStore
	Conns     AnalysisConnectionResolver
	Agent     AgentPromptRunner
	Probe     RepoStateProbe
	// Caps may be nil; the run then counts as an agent without read-only enforcement.
	Caps     DevServerCapabilityReader
	Projects ProjectContextReader
	Writer   *AnalysisResultWriter
	Settings AnalysisSettings
}

func NewRunAgentReadonlyAnalysis(d AgentReadonlyDeps) *RunAgentReadonlyAnalysis {
	return &RunAgentReadonlyAnalysis{requests: d.Requests, solutions: d.Solutions, conns: d.Conns, agent: d.Agent, probe: d.Probe, caps: d.Caps,
		projects: d.Projects, writer: d.Writer, settings: d.Settings.withDefaults()}
}

// Equal compares branch and the sorted file states; HEAD is not part of the snapshot because git-gateway does not return it.
func (s RepoSnapshot) Equal(o RepoSnapshot) bool {
	if s.Branch != o.Branch || len(s.Files) != len(o.Files) {
		return false
	}
	count := map[RepoFileState]int{}
	for _, f := range s.Files {
		count[f]++
	}
	for _, f := range o.Files {
		count[f]--
	}
	for _, c := range count {
		if c != 0 {
			return false
		}
	}
	return true
}

func (a *RunAgentReadonlyAnalysis) Execute(ctx context.Context, run domain.AnalysisRun) error {
	req, err := a.requests.Get(ctx, run.RequestID)
	if err != nil {
		return err
	}
	kind := domain.SolutionKind(run.Kind)
	conn, err := a.conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return newFailure(run, domain.RunErrAnalysisNoConn, "no dev server is connected for this project")
		}
		return err
	}
	if conn.RepoPath == "" {
		return newFailure(run, domain.RunErrNoRepoPath, "the project connection has no repository path")
	}

	enforced, err := a.enforcement(ctx, conn)
	if err != nil {
		return newFailure(run, domain.RunErrAgentTooOld, "the dev server agent cannot enforce read-only mode")
	}
	run.Enforcement = domain.EnforcementPromptOnly
	if enforced {
		run.Enforcement = domain.EnforcementAgent
	}

	before, repoCheck := a.snapshot(ctx, conn.WorktreeID)
	run.RepoCheck = repoCheck

	in := SolutionPromptInput{
		Kind: kind, Request: req, Feedback: run.Feedback, MinOptions: domain.MinOptionsFor(req.Type),
		PriorArtifacts: priorArtifacts(ctx, a.solutions, req.ID, kind, run.SolutionID),
	}
	in.ProjectName, in.RepoURL = projectFacts(ctx, a.projects, req.ProjectID)
	timeoutMS := a.settings.AgentTimeoutMS
	if req.Type == domain.RequestTypeHotfix {
		timeoutMS = a.settings.HotfixTimeoutMS
	}

	var lastRaw string
	for {
		res, err := a.agent.ExecPrompt(ctx, conn, AgentPromptInput{
			StepID: run.ID, Prompt: BuildSolutionPrompt(in), RepoPath: conn.RepoPath, RequestID: req.ID, ProjectID: req.ProjectID,
			TimeoutMS: timeoutMS, ReadOnlyEnforced: enforced,
		})
		if err != nil {
			return agentCallFailure(run, err)
		}
		lastRaw = res.Stdout
		if f := a.checkRun(ctx, &run, res, enforced, before, repoCheck, conn.WorktreeID); f != nil {
			f.Raw = lastRaw
			return f
		}
		doc, verr := parseAnalysisOutput(kind, res.Stdout, in.MinOptions)
		if verr == nil {
			return a.writer.Persist(ctx, PersistAnalysisInput{Run: run, Doc: doc, Raw: res.Stdout})
		}
		if rerr := run.RecordAttempt(); rerr != nil {
			f := newFailure(run, domain.RunErrAnalysisInvalid, "the agent did not return a valid document after a retry: "+verr.Error())
			f.Raw = lastRaw
			return f
		}
		in.RetryNote = verr.Error()
	}
}

// enforcement picks the route. A missing or unreadable capability profile means an agent that cannot be trusted to enforce.
func (a *RunAgentReadonlyAnalysis) enforcement(ctx context.Context, conn AnalysisConnection) (bool, error) {
	var profile domain.DevServerCapability
	if a.settings.UseAgentReadonlyFlag && a.caps != nil {
		got, err := a.caps.Get(ctx, DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}, false)
		switch {
		case err == nil:
			profile = got
		case errors.Is(err, ErrCapabilityNotAvailable):
		default:
			slog.WarnContext(ctx, "capability profile unreadable; treating the agent as prompt-only", slog.Any("error", err))
		}
	}
	route, err := domain.SelectReadonlyRoute(profile, a.settings.RequireEnforcedReadonly)
	if err != nil {
		return false, err
	}
	return route == domain.RouteAgentEnforced, nil
}

// snapshot returns repo_check=skipped when git-gateway cannot be asked, instead of pretending the repo was checked.
func (a *RunAgentReadonlyAnalysis) snapshot(ctx context.Context, worktreeID string) (RepoSnapshot, string) {
	if a.probe == nil || worktreeID == "" {
		slog.WarnContext(ctx, "read-only analysis runs without a repository comparison: no worktree id")
		return RepoSnapshot{}, domain.RepoCheckSkipped
	}
	snap, err := a.probe.Snapshot(ctx, worktreeID)
	if err != nil {
		if !errors.Is(err, ErrProbeUnavailable) {
			slog.WarnContext(ctx, "repository snapshot failed; skipping the comparison", slog.Any("error", err))
		}
		return RepoSnapshot{}, domain.RepoCheckSkipped
	}
	return snap, domain.RepoCheckClean
}

// checkRun turns an agent result into a failure when the run itself went wrong or the repo changed.
func (a *RunAgentReadonlyAnalysis) checkRun(ctx context.Context, run *domain.AnalysisRun, res AgentPromptResult, enforced bool, before RepoSnapshot, repoCheck, worktreeID string) *AnalysisFailure {
	if res.TimedOut {
		return newFailure(*run, domain.RunErrAnalysisTimeout, "the agent run timed out")
	}
	if res.ExitCode != 0 {
		return newFailure(*run, domain.RunErrAnalysisAgent, fmt.Sprintf("the agent exited with code %d", res.ExitCode))
	}
	for _, w := range res.Warnings {
		if w == "READONLY_VIOLATION" {
			run.RepoCheck = domain.RepoCheckModified
			return newFailure(*run, domain.RunErrRepoModified, "the agent reported changes in a read-only run; the result was discarded")
		}
	}
	if enforced {
		if res.AppliedAccessMode != "readonly" {
			return newFailure(*run, domain.RunErrReadonlyNotActive, "the agent did not confirm read-only mode")
		}
		if res.HeadMoved || res.ChangedFiles > 0 {
			run.RepoCheck = domain.RepoCheckModified
			return newFailure(*run, domain.RunErrRepoModified, "the agent reported changes in a read-only run; the result was discarded")
		}
	}
	if repoCheck == domain.RepoCheckClean {
		after, err := a.probe.Snapshot(ctx, worktreeID)
		if err != nil {
			slog.WarnContext(ctx, "repository snapshot after the run failed; comparison skipped", slog.Any("error", err))
			run.RepoCheck = domain.RepoCheckSkipped
			return nil
		}
		if !before.Equal(after) {
			run.RepoCheck = domain.RepoCheckModified
			slog.WarnContext(ctx, "read-only analysis changed the repository", slog.String("run_id", run.ID))
			return newFailure(*run, domain.RunErrRepoModified, "the repository changed during a read-only run; the result was discarded")
		}
	}
	return nil
}

func agentCallFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	switch {
	case errors.Is(err, ErrAgentReadonlyUnsupported):
		return newFailure(run, domain.RunErrAgentTooOld, "the agent refused read-only mode")
	case errors.Is(err, ErrNoDevServer):
		return newFailure(run, domain.RunErrAnalysisNoConn, "no dev server could run the analysis")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrClassifierTimeout):
		return newFailure(run, domain.RunErrAnalysisTimeout, "the agent run timed out")
	}
	slog.Warn("agent.execPrompt failed", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, domain.RunErrAnalysisAgent, "the agent call failed")
}
