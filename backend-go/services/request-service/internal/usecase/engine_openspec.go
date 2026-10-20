package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrEngineStepNotSupported is returned for a step the openspec engine does not own: its Solution comes from the
// Solution flow, and only the Plan is authored as an OpenSpec change.
var ErrEngineStepNotSupported = errors.New("the openspec engine does not generate this step")

// OpenSpecWriteProfile is the permission profile of the authoring agent: file edits inside the worktree and the openspec
// and tdd-agent status commands, never full trust.
const OpenSpecWriteProfile = "openspec-author"

type OpenSpecEngineDeps struct {
	Gate      *EngineReadinessGate
	Workspace ProposalWorkspace
	Agent     AgentPromptRunner
	Conns     AnalysisConnectionResolver
	// Caps may be nil; the engine then cannot prove the agent is safe to run and refuses.
	Caps     DevServerCapabilityReader
	Settings AnalysisSettings
	// Observer may be nil; it receives the knowledge tool calls of each agent call.
	Observer EngineObserver
	// NewNonce fences the agent's result block; production uses a random value per attempt.
	NewNonce func() string
}

type openspecEngine struct {
	gate *EngineReadinessGate
	ws   ProposalWorkspace
	d    OpenSpecEngineDeps
}

func NewOpenSpecEngine(d OpenSpecEngineDeps) *openspecEngine {
	d.Settings = d.Settings.withDefaults()
	return &openspecEngine{gate: d.Gate, ws: d.Workspace, d: d}
}

func (e *openspecEngine) Name() domain.EngineName { return domain.EngineOpenSpec }

func (e *openspecEngine) Preflight(ctx context.Context, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	return e.gate.Check(ctx, p.ProjectID, settings)
}

func (e *openspecEngine) GenerateAnalysis(context.Context, ProjectRef, AnalysisInput) (AnalysisOutput, error) {
	return AnalysisOutput{}, ErrEngineStepNotSupported
}

func (e *openspecEngine) OnRequestCompleted(context.Context, ProjectRef, CompletionInput) error {
	return nil
}

// GeneratePlan returns the raw tasks.md the agent wrote; callers that need the worktree, the agent's report and the
// retry loop use GenerateProposal.
func (e *openspecEngine) GeneratePlan(ctx context.Context, p ProjectRef, in PlanInput) (string, error) {
	return "", fmt.Errorf("%w: use GenerateProposal", ErrEngineStepNotSupported)
}

// ProposalInput is one attempt of the authoring run.
type ProposalInput struct {
	Run     domain.AnalysisRun
	Request domain.Request
	// ChangeID is the directory under openspec/changes/; the worker fixes it at the Request's first generation.
	ChangeID string
	Shape    domain.PlanShape
	Limits   domain.PlanLimits
	// Analysis is the approved Solution as text; Feedback the reviewer's guidance on a regenerate.
	Analysis, Feedback string
	// PreviousFiles are the files of the version being revised, relative to the change directory; empty on a first run.
	PreviousFiles []string
	ProjectName   string
	RepoURL       string
	// Settings is the project's engine setting, for the readiness check.
	Settings *domain.ProjectEngineSettings
	// RetryNote and Workspace are set on the second attempt: the agent fixes the files already in place.
	RetryNote string
	Workspace *Workspace
	// OnWorkspace is told about the worktree as soon as it exists, so a run that fails afterwards still leaves a record the
	// cleanup can find. It must not fail the run.
	OnWorkspace func(Workspace)
	// Stats collects the tool calls and usage of every agent call of the run, failed attempts included.
	Stats *domain.RunStats
}

// AgentReport is what the agent says about its own run in the result block. Orca treats it as data to show and to
// cross-check, never as a fact: validation is re-run on the files.
type AgentReport struct {
	Present       bool            `json:"-"`
	ChangeID      string          `json:"changeId"`
	Phases        int             `json:"phases"`
	Tasks         int             `json:"tasks"`
	Validation    string          `json:"validation"`
	OpenQuestions int             `json:"openQuestions"`
	Knowledge     KnowledgeReport `json:"knowledge"`
}

type KnowledgeReport struct {
	Available bool     `json:"available"`
	Snapshot  string   `json:"snapshot"`
	Read      []string `json:"read"`
	Gaps      []string `json:"gaps"`
	Conflicts []string `json:"conflicts"`
}

type ProposalResult struct {
	ChangeID  string
	Ref       string
	Workspace Workspace
	// TasksMD is the raw file the Plan is read from.
	TasksMD string
	Report  AgentReport
	// SkillHash names the installed knowledge skills the agent worked under; empty when the agent did not report them all.
	SkillHash string
	// KnowledgeRequested says whether the agent was given the project's knowledge graph for this attempt.
	KnowledgeRequested bool
}

const tasksMDName = "tasks.md"

// GenerateProposal runs the agent once. A returned error is *AnalysisFailure: the run cannot continue.
func (e *openspecEngine) GenerateProposal(ctx context.Context, in ProposalInput) (ProposalResult, error) {
	run, req := in.Run, in.Request
	conn, err := e.d.Conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return ProposalResult{}, newFailure(run, domain.RunErrAnalysisNoConn, "no dev server is connected for this project")
		}
		return ProposalResult{}, err
	}
	if conn.RepoPath == "" {
		return ProposalResult{}, newFailure(run, domain.RunErrNoRepoPath, "the project's default repository has no folder path on its dev server")
	}
	profile, err := e.capability(ctx, conn)
	if err != nil {
		return ProposalResult{}, newFailure(run, domain.RunErrSpecAgentOld, "the dev server agent cannot run the openspec engine safely: "+err.Error())
	}

	ws := in.Workspace
	if ws == nil {
		fresh, f := e.prepare(ctx, run, req, in, conn)
		if f != nil {
			return ProposalResult{}, f
		}
		ws = &fresh
	}

	changeID, ref := in.ChangeID, OpenSpecRef(req.Number)
	if !domain.ValidChangeID(changeID) {
		return ProposalResult{}, newFailure(run, domain.RunErrInternal, "the change id is not valid")
	}
	useKnowledge := domain.SupportsKnowledgeHub(profile)
	res, requested, f := e.runAgent(ctx, run, req, in, conn, *ws, changeID, ref, useKnowledge)
	if f != nil {
		return ProposalResult{}, f
	}
	if f := e.checkRun(run, res); f != nil {
		return ProposalResult{}, f
	}
	if f := e.checkScope(ctx, run, *ws, changeID); f != nil {
		return ProposalResult{}, f
	}
	raw, err := e.ws.ReadFile(ctx, *ws, "openspec/changes/"+changeID+"/"+tasksMDName, MaxProposalReadBytes)
	if err != nil {
		f := newFailure(run, domain.RunErrSpecInvalidOutput, "the agent did not leave a readable "+tasksMDName+" in the change")
		f.Raw = res.Stdout
		return ProposalResult{}, f
	}
	return ProposalResult{ChangeID: changeID, Ref: ref, Workspace: *ws, TasksMD: string(raw), Report: parseAgentReport(res.ResultBlock), KnowledgeRequested: requested, SkillHash: profile.SkillHash(domain.KnowledgeSkillNames)}, nil
}

// capability reads the agent's profile; an unreadable profile means the engine cannot show the agent is safe, so it refuses.
func (e *openspecEngine) capability(ctx context.Context, conn AnalysisConnection) (domain.DevServerCapability, error) {
	if e.d.Caps == nil {
		return domain.DevServerCapability{}, errors.New("no capability information is available")
	}
	c, err := e.d.Caps.Get(ctx, DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}, false)
	if err != nil {
		return domain.DevServerCapability{}, fmt.Errorf("the agent's capabilities could not be read: %w", err)
	}
	if !domain.SupportsOpenSpecAuthoring(c) {
		return domain.DevServerCapability{}, errors.New("the agent lacks agent.execPrompt.permissionProfile or a reporting feature; update it")
	}
	return c, nil
}

// prepare runs the readiness check and gives the Request a clean worktree on its proposal branch.
func (e *openspecEngine) prepare(ctx context.Context, run domain.AnalysisRun, req domain.Request, in ProposalInput, conn AnalysisConnection) (Workspace, *AnalysisFailure) {
	if e.gate != nil {
		rep, err := e.gate.Check(ctx, req.ProjectID, in.Settings)
		if err != nil {
			slog.WarnContext(ctx, "openspec readiness check could not run", slog.String("run_id", run.ID), slog.Any("error", err))
			return Workspace{}, newFailure(run, domain.RunErrSpecAgentFailed, "the openspec readiness check could not run")
		}
		if !rep.OK {
			f := rep.Failures[0]
			return Workspace{}, newFailure(run, f.Code, f.Message+". "+f.FixHint)
		}
	}
	ws, err := e.ws.Ensure(ctx, EnsureWorkspaceInput{
		TenantID: run.TenantID, ProjectID: req.ProjectID, RequestID: req.ID, RequestNumber: req.Number, ChangeID: in.ChangeID, BaseRef: req.BaseRef,
	})
	if err != nil {
		return Workspace{}, workspaceFailure(run, err)
	}
	if in.OnWorkspace != nil {
		in.OnWorkspace(ws)
	}
	if err := e.ws.DiscardAll(ctx, ws); err != nil {
		return Workspace{}, workspaceFailure(run, err)
	}
	return ws, nil
}

func (e *openspecEngine) runAgent(ctx context.Context, run domain.AnalysisRun, req domain.Request, in ProposalInput, conn AnalysisConnection,
	ws Workspace, changeID, ref string, useKnowledge bool) (AgentPromptResult, bool, *AnalysisFailure) {
	min, max := TaskRangeFor(req.Size, in.Limits)
	call := func(withKnowledge bool) (AgentPromptResult, error) {
		nonce := e.nonce()
		prompt, perr := RenderOpenSpecPlanPrompt(OpenSpecPlanPromptInput{
			Request: req, Ref: ref, ChangeID: changeID, Shape: in.Shape, Limits: in.Limits, MinTasks: min, MaxTasks: max,
			ProjectName: in.ProjectName, RepoURL: in.RepoURL, Analysis: in.Analysis, Feedback: in.Feedback, PreviousFiles: in.PreviousFiles,
			KnowledgeAvailable: withKnowledge, ResultNonce: nonce, RetryNote: in.RetryNote, Language: e.d.Settings.OutputLanguageFor(req),
		})
		if perr != nil {
			return AgentPromptResult{}, perr
		}
		input := AgentPromptInput{
			StepID: run.ID, Prompt: prompt, RepoPath: ws.Path, RequestID: req.ID, ProjectID: req.ProjectID,
			TimeoutMS: e.d.Settings.SpecTimeoutMS, WriteProfile: OpenSpecWriteProfile, ResultNonce: nonce,
		}
		if withKnowledge {
			// The agent reads the graph from the project checkout, where .tdd-agent/ lives, not from the proposal worktree.
			input.KnowledgeRoot = conn.RepoPath
		}
		res, err := e.d.Agent.ExecPrompt(ctx, conn, input)
		if err == nil {
			if in.Stats != nil {
				*in.Stats = in.Stats.Merge(res.Stats)
			}
			observerOrNoop(e.d.Observer).KnowledgeToolCalls(res.Stats)
		}
		return res, err
	}
	res, err := call(useKnowledge)
	if err != nil && useKnowledge && errors.Is(err, ErrAgentKnowledgeHubUnavailable) {
		// The run did not start, so asking again without the graph costs nothing; the prompt says the graph is absent.
		slog.WarnContext(ctx, "knowledge hub unavailable; authoring the change without it", slog.String("run_id", run.ID), slog.Any("error", err))
		useKnowledge = false
		res, err = call(false)
	}
	if err != nil {
		return AgentPromptResult{}, useKnowledge, specAgentFailure(run, err)
	}
	return res, useKnowledge, nil
}

func (e *openspecEngine) nonce() string {
	if e.d.NewNonce != nil {
		return e.d.NewNonce()
	}
	return randomNonce()
}

func (e *openspecEngine) checkRun(run domain.AnalysisRun, res AgentPromptResult) *AnalysisFailure {
	switch {
	case res.TimedOut:
		return newFailure(run, domain.RunErrSpecTimeout, "the agent run timed out")
	case res.ExitCode != 0:
		f := newFailure(run, domain.RunErrSpecAgentFailed, fmt.Sprintf("the agent exited with code %d", res.ExitCode))
		f.Raw = res.Stderr
		return f
	}
	return nil
}

// checkScope fails the run when the agent touched anything outside its change directory. The worktree is kept so the
// evidence stays for a person to read.
func (e *openspecEngine) checkScope(ctx context.Context, run domain.AnalysisRun, ws Workspace, changeID string) *AnalysisFailure {
	changed, err := e.ws.ChangedPaths(ctx, ws)
	if err != nil {
		return workspaceFailure(run, err)
	}
	paths := make([]string, 0, len(changed))
	for _, c := range changed {
		paths = append(paths, c.Path)
		if c.OldPath != "" {
			paths = append(paths, c.OldPath)
		}
	}
	if out := CheckProposalScope(paths, changeID); len(out) > 0 {
		shown := out
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return newFailure(run, domain.RunErrSpecOutOfScope, "the agent changed files outside openspec/changes/"+changeID+"/: "+strings.Join(shown, ", "))
	}
	return nil
}

func specAgentFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	switch {
	case errors.Is(err, errPromptBuild):
		return promptBuildFailure(run, err)
	case errors.Is(err, ErrNoDevServer):
		return newFailure(run, domain.RunErrAnalysisNoConn, "no dev server could run the authoring agent")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrClassifierTimeout):
		return newFailure(run, domain.RunErrSpecTimeout, "the agent run timed out")
	case errors.Is(err, ErrAgentKnowledgeHubUnavailable):
		return newFailure(run, domain.RunErrSpecAgentFailed, "the agent could not load the knowledge hub")
	}
	slog.Warn("openspec authoring agent failed", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, domain.RunErrSpecAgentFailed, "the agent call failed")
}

// workspaceFailure keeps the code of a workspace error that already carries one, so the run row says why.
func workspaceFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	if domain.IsBaseRefNotFound(err) {
		return newFailure(run, domain.RunErrBaseRefNotFound, baseRefGoneMessage(err))
	}
	if code := errorCode(err); strings.HasPrefix(code, "REQUEST_ENGINE_") || strings.HasPrefix(code, "REQUEST_OPENSPEC_") {
		return newFailure(run, code, "the proposal workspace failed")
	}
	slog.Warn("proposal workspace failed", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, domain.RunErrSpecAgentFailed, "the proposal workspace failed")
}

func parseAgentReport(block []byte) AgentReport {
	if len(block) == 0 {
		return AgentReport{}
	}
	var r AgentReport
	if err := json.Unmarshal(block, &r); err != nil {
		return AgentReport{}
	}
	r.Present = true
	return r
}

// baseRefGoneMessage says which ref is gone and what exists instead, so the activity feed tells the person what to change.
// The ref is a branch name, not Request content.
func baseRefGoneMessage(err error) string {
	details, _ := domain.BaseRefDetails(err)
	msg := fmt.Sprintf("the base branch %q does not exist in the repository any more; choose another base ref and generate the Plan again", details["ref"])
	if s := details["suggestions"]; s != "" {
		msg += " (available: " + strings.ReplaceAll(s, ",", ", ") + ")"
	}
	return msg
}
