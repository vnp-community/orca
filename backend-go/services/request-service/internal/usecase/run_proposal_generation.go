package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ProposalEngine is the part of the openspec engine the worker drives: one authoring attempt in a (possibly reused) worktree.
type ProposalEngine interface {
	GenerateProposal(ctx context.Context, in ProposalInput) (ProposalResult, error)
}

// BundleChecker looks at the finished change for faults a parse of tasks.md cannot see (T2.5). Nil means no extra check.
type BundleChecker interface {
	Check(ctx context.Context, in BundleCheckInput) ([]BundleFinding, error)
}

type BundleCheckInput struct {
	Request   domain.Request
	Result    ProposalResult
	Proposal  domain.PlanProposal
	Workspace ProposalWorkspace
}

// BundleFinding is one fault in the change, worded so the agent can fix it on a retry.
type BundleFinding struct {
	Code    string
	Message string
}

// RunProposalGeneration is the worker of agent_proposal plan runs: an agent writes an OpenSpec change, and the Plan is read
// from its tasks.md. It writes nothing to task-service; a person reviews the proposal and CommitPlan writes it.
type RunProposalGeneration struct {
	requests       RequestRepository
	solutions      SolutionStore
	registry       EngineRegistry
	projects       ProjectContextReader
	policies       PlanPolicies
	engineSettings EngineSettingsRepository
	checker        BundleChecker
	workspace      ProposalWorkspace
	changes        OpenSpecChangeRepository
	revisions      OpenSpecChangeRevisionRepository
	timing         AnalysisSettings
	newID          func() string
	runs           AnalysisRunStore
	tx             TxRunner
	limits         domain.PlanLimits
	owner          string
	now            func() time.Time
	observer       EngineObserver
}

var _ AnalysisWorker = (*RunProposalGeneration)(nil)

type RunProposalGenerationDeps struct {
	Requests  RequestRepository
	Solutions SolutionStore
	// Registry holds the openspec engine; it must also implement ProposalEngine.
	Registry EngineRegistry
	Projects ProjectContextReader
	Policies PlanPolicies
	// EngineSettings may be nil; the readiness check then uses the server's default minimum version.
	EngineSettings EngineSettingsRepository
	// Settings supplies SpecTotalTimeoutMS, the bound of one run with its retry.
	Settings  AnalysisSettings
	Checker   BundleChecker
	Workspace ProposalWorkspace
	// Changes stores what the change was built from; the proposal cannot be recorded without it.
	Changes OpenSpecChangeRepository
	// Revisions keeps one version of the change per succeeded run, stored in the same transaction as the change.
	Revisions OpenSpecChangeRevisionRepository
	Runs      AnalysisRunStore
	Tx        TxRunner
	Limits    domain.PlanLimits
	Owner     string
	// Observer may be nil; it receives the structure problems found in an authored change.
	Observer EngineObserver
}

func NewRunProposalGeneration(d RunProposalGenerationDeps) *RunProposalGeneration {
	return &RunProposalGeneration{requests: d.Requests, solutions: d.Solutions, registry: d.Registry, projects: d.Projects, policies: d.Policies,
		engineSettings: d.EngineSettings, timing: d.Settings.withDefaults(), checker: d.Checker, workspace: d.Workspace, changes: d.Changes, revisions: d.Revisions, newID: uuid.NewString, runs: d.Runs, tx: d.Tx, limits: d.Limits, owner: d.Owner, now: func() time.Time { return time.Now().UTC() }, observer: d.Observer}
}

func (w *RunProposalGeneration) Execute(ctx context.Context, run domain.AnalysisRun) error {
	req, err := w.requests.Get(ctx, run.RequestID)
	if err != nil {
		return err
	}
	if req.Status != domain.RequestStatusPlanning {
		return newFailure(run, planRunMoved, "request left planning while the Plan was being written")
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return err
	}
	shape := domain.PlanShapeFor(flow, req.Size)
	if shape == domain.ShapeNone {
		return newFailure(run, "REQUEST_PLAN_NOT_APPLICABLE", "this request type has no Plan")
	}

	existing, hasChange, err := w.changes.GetByRequest(ctx, req.ID)
	if err != nil {
		return err
	}
	// The change id is fixed at the first generation: a regenerate after the title was edited must reuse the same directory and branch.
	changeID := existing.ChangeID
	if !hasChange || !domain.ValidChangeID(changeID) {
		changeID = domain.NewChangeID(req.Number, req.Title)
	}
	in := ProposalInput{Run: run, Request: req, ChangeID: changeID, Shape: shape, Limits: w.limits, Feedback: run.Feedback, Settings: w.projectEngineSettings(ctx, req.ProjectID)}
	in.PreviousFiles = previousChangeFiles(existing, hasChange)
	var stats domain.RunStats
	in.Stats = &stats
	in.OnWorkspace = func(ws Workspace) { w.recordWorkspace(ctx, req, changeID, ws) }
	in.ProjectName, in.RepoURL = projectFacts(ctx, w.projects, req.ProjectID)
	if sol, ok := latestApprovedAnalysis(ctx, w.solutions, req.ID); ok {
		in.Analysis = approvedAnalysisText(sol)
	}
	policy := w.policies.PolicyFor(req.Type)

	engine, err := w.proposalEngine()
	if err != nil {
		return newFailure(run, RunErrEngineUnavailable, err.Error())
	}
	// Each attempt has its own budget inside the agent call; this bounds the run with its retry.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(w.timing.SpecTotalTimeoutMS)*time.Millisecond)
	defer cancel()
	for {
		res, err := engine.GenerateProposal(ctx, in)
		run.Stats = statsPtr(stats)
		if err != nil {
			if f, ok := asFailure(err); ok && f.Stats == nil {
				f.Stats = run.Stats
			}
			return err
		}
		in.Workspace = &res.Workspace // the retry fixes the files in place
		proposal, problems, fatal := w.assemble(ctx, req, shape, policy, res)
		if fatal != nil {
			f := newFailure(run, errorCode(fatal), fatal.Error())
			f.Raw = res.TasksMD
			return f
		}
		if len(problems) == 0 {
			if !res.Report.Present {
				// REQUEST_SPEC_REPORT_MISSING: worth knowing, never a reason to refuse a change that passed every check.
				slog.WarnContext(ctx, "the authoring agent left no result block", slog.String("run_id", run.ID), slog.String("code", "REQUEST_SPEC_REPORT_MISSING"))
			}
			return w.finish(ctx, run, req, res, proposal, existing, hasChange)
		}
		if rerr := run.RecordAttempt(); rerr != nil {
			f := newFailure(run, domain.RunErrSpecInvalidOutput, "the agent did not leave a valid change after a retry: "+problems[0])
			f.Raw = strings.Join(problems, "\n")
			return f
		}
		in.RetryNote = strings.Join(problems, "\n")
	}
}

// recordWorkspace notes the worktree on the change row the moment it exists. A run that fails later leaves that row as
// "preparing", which is how the cleanup finds a worktree no successful run ever recorded. Best effort: it never fails the run.
// The row is read again rather than taken from the run's start: the reservation made before the worktree was created is newer.
func (w *RunProposalGeneration) recordWorkspace(ctx context.Context, req domain.Request, changeID string, ws Workspace) {
	c, found, err := w.changes.GetByRequest(ctx, req.ID)
	if err != nil {
		slog.WarnContext(ctx, "could not record the proposal worktree", slog.String("request_id", req.ID), slog.Any("error", err))
		return
	}
	if !found {
		c = domain.OpenSpecChange{ID: w.newID(), RequestID: req.ID, ChangeID: changeID, Branch: ws.Branch, Status: domain.ChangeStatusPreparing, SyncState: domain.SyncStateInSync}
	}
	if c.WorkspaceID == ws.WorktreeID {
		return
	}
	c.WorkspaceID = ws.WorktreeID
	if _, err := w.changes.Upsert(ctx, c); err != nil {
		slog.WarnContext(ctx, "could not record the proposal worktree", slog.String("request_id", req.ID), slog.Any("error", err))
	}
}

// proposalEngine finds the openspec engine in the registry; a server that did not register one cannot run these runs.
func (w *RunProposalGeneration) proposalEngine() (ProposalEngine, error) {
	if w.registry == nil {
		return nil, fmt.Errorf("this server has no engine registry")
	}
	e, err := w.registry.For(domain.EngineOpenSpec)
	if err != nil {
		return nil, fmt.Errorf("this server has no worker for the openspec engine")
	}
	pe, ok := e.(ProposalEngine)
	if !ok {
		return nil, fmt.Errorf("the registered openspec engine cannot author a change")
	}
	return pe, nil
}

func (w *RunProposalGeneration) projectEngineSettings(ctx context.Context, projectID string) *domain.ProjectEngineSettings {
	if w.engineSettings == nil {
		return nil
	}
	s, found, err := w.engineSettings.Get(ctx, projectID)
	if err != nil || !found {
		return nil
	}
	return &s
}

// assemble turns the change into a validated PlanProposal, or returns what is wrong in words the agent can act on.
// fatal is a problem a retry cannot fix (the project's state, not the agent's output).
func (w *RunProposalGeneration) assemble(ctx context.Context, req domain.Request, shape domain.PlanShape, policy domain.TypePolicy, res ProposalResult) (proposal domain.PlanProposal, problems []string, fatal error) {
	proposal, violations := domain.ParsePlanRegion(res.TasksMD, res.Ref)
	if len(violations) > 0 {
		for _, v := range violations {
			observerOrNoop(w.observer).SpecViolation(v.Code)
		}
		return domain.PlanProposal{}, violationLines(violations), nil
	}
	proposal.Title = req.Title
	proposal.Summary = "OpenSpec change " + res.ChangeID
	proposal, err := FillTaskPrompts(proposal, TaskPromptContext{Ref: res.Ref, ChangeID: res.ChangeID, Branch: res.Workspace.Branch})
	if err != nil {
		return domain.PlanProposal{}, nil, err
	}
	proposal = domain.NormalizePlanProposal(proposal)
	if err := domain.ValidatePlanProposal(req.Type, shape, w.limits, proposal); err != nil {
		if !planErrorIsModelFault(err) {
			return domain.PlanProposal{}, nil, err
		}
		return domain.PlanProposal{}, []string{err.Error()}, nil
	}
	if err := policy.PlanPreconditions(ctx, req, proposal); err != nil {
		if !planErrorIsModelFault(err) {
			return domain.PlanProposal{}, nil, err
		}
		return domain.PlanProposal{}, []string{err.Error()}, nil
	}
	if w.checker != nil {
		findings, err := w.checker.Check(ctx, BundleCheckInput{Request: req, Result: res, Proposal: proposal, Workspace: w.workspace})
		if err != nil {
			return domain.PlanProposal{}, []string{"the change could not be checked: " + err.Error()}, nil
		}
		if len(findings) > 0 {
			out := make([]string, len(findings))
			for i, f := range findings {
				observerOrNoop(w.observer).SpecViolation(f.Code)
				out[i] = f.Code + ": " + f.Message
			}
			return domain.PlanProposal{}, out, nil
		}
	}
	return proposal, nil, nil
}

func violationLines(v []domain.Violation) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		out = append(out, fmt.Sprintf("tasks.md line %d %s: %s", x.Line, x.Code, x.Message))
		if len(out) == 20 {
			break
		}
	}
	return out
}

// Caps on what is kept of the change in the database: every file is read as a proposal document, and the total stays small
// enough for a row. A file beyond the budget is kept as a marker with its size and hash, so the list and the digest stay whole.
const (
	maxBundleBytes     = 1 << 20
	maxBundleDepth     = 6
	maxBundleFiles     = 200
	maxReportedEntries = 200
	maxReportedRunes   = 500
)

var errNothingToCommit = errors.New("the worktree has nothing to commit")

// finish commits the change to its branch, reads the files back, records what the change was built from and stores the
// proposal on the run, the last two in one transaction. The commit comes first: a change that is not on the branch must
// not be recorded as ready.
func (w *RunProposalGeneration) finish(ctx context.Context, run domain.AnalysisRun, req domain.Request, res ProposalResult, p domain.PlanProposal, prev domain.OpenSpecChange, hasPrev bool) error {
	encoded, err := EncodePlanProposal(p)
	if err != nil {
		return err
	}
	sha, bundle, redacted, err := w.commitAndRead(ctx, req, res)
	if errors.Is(err, errNothingToCommit) && hasPrev && prev.Status == domain.ChangeStatusReady {
		// A revision that leaves every document as it was would only reopen approval on the same Plan.
		return newFailure(run, domain.RunErrPlanRevisionNoChange, "the agent did not change any document")
	}
	if err != nil {
		slog.WarnContext(ctx, "could not commit the proposal to its branch", slog.String("run_id", run.ID), slog.Any("error", err))
		return newFailure(run, domain.RunErrSpecAgentFailed, "the change could not be committed to its branch")
	}
	change := domain.OpenSpecChange{
		ID: w.newID(), RequestID: req.ID, ChangeID: res.ChangeID, Branch: res.Workspace.Branch, Status: domain.ChangeStatusReady,
		SyncState: domain.SyncStateInSync, Commit: sha, TasksMDDigest: domain.PlanRegionDigest(res.TasksMD),
		KGAvailable: res.Report.Present && res.Report.Knowledge.Available && res.KnowledgeRequested, KGSnapshot: clip(res.Report.Knowledge.Snapshot, maxReportedRunes),
		KGReadIDs: boundedList(res.Report.Knowledge.Read), KGGaps: boundedList(res.Report.Knowledge.Gaps), KGConflicts: boundedList(res.Report.Knowledge.Conflicts),
		SkillHash: res.SkillHash, Bundle: bundle, RedactedSecrets: redacted, WorkspaceID: res.Workspace.WorktreeID,
	}
	if hasPrev {
		change.ID = prev.ID
	}
	return w.tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := w.changes.Upsert(ctx, change); err != nil {
			return err
		}
		if err := w.appendRevision(ctx, run, change); err != nil {
			return err
		}
		return finishPlanRun(ctx, w.tx, w.runs, w.owner, w.now(), run, encoded)
	})
}

// commitAndRead commits every changed path of the (already scope-checked) worktree and reads the change's files back as the
// masked copy the database keeps, returning how many secrets that masked.
func (w *RunProposalGeneration) commitAndRead(ctx context.Context, req domain.Request, res ProposalResult) (string, map[string]string, int, error) {
	changed, err := w.workspace.ChangedPaths(ctx, res.Workspace)
	if err != nil {
		return "", nil, 0, err
	}
	prefix := "openspec/changes/" + res.ChangeID + "/"
	var paths []string
	for _, c := range changed {
		if !strings.HasPrefix(c.Path, prefix) {
			return "", nil, 0, fmt.Errorf("a path outside the change appeared after the scope check: %s", c.Path)
		}
		paths = append(paths, c.Path)
	}
	if len(paths) == 0 {
		return "", nil, 0, errNothingToCommit
	}
	sha, err := w.workspace.Commit(ctx, res.Workspace, "proposal: "+res.Ref+" "+clip(req.Title, 100), paths)
	if err != nil {
		return "", nil, 0, err
	}
	// The whole change, not only the paths this run touched: a regenerated change leaves the files it did not rewrite on the
	// branch, and a bundle without them would hide capabilities from the reader and from the digest the approval is bound to.
	all, err := w.changeFiles(ctx, res.Workspace, prefix)
	if err != nil {
		return "", nil, 0, err
	}
	files := make([]bundleFile, 0, len(all))
	for _, p := range all {
		b, err := w.workspace.ReadFile(ctx, res.Workspace, p, MaxProposalReadBytes)
		if err != nil {
			return "", nil, 0, fmt.Errorf("read back %s: %w", p, err)
		}
		files = append(files, bundleFile{Path: strings.TrimPrefix(p, prefix), Content: b})
	}
	bundle, redacted := redactBundle(files, maxBundleBytes)
	return sha, bundle, redacted, nil
}

// changeFiles lists every file under the change directory, directories before files and each level in name order, bounded in
// depth and count so a runaway tree cannot grow the walk.
func (w *RunProposalGeneration) changeFiles(ctx context.Context, ws Workspace, root string) ([]string, error) {
	var files []string
	queue := []string{strings.TrimSuffix(root, "/")}
	for depth := 0; depth < maxBundleDepth && len(queue) > 0; depth++ {
		var next []string
		for _, dir := range queue {
			entries, err := w.workspace.ListDir(ctx, ws, dir)
			if err != nil {
				return nil, fmt.Errorf("list %s: %w", dir, err)
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
			for _, e := range entries {
				switch {
				case e.IsDir:
					next = append(next, dir+"/"+e.Name)
				case len(files) < maxBundleFiles:
					files = append(files, dir+"/"+e.Name)
				}
			}
		}
		queue = next
	}
	return files, nil
}

func omittedMarker(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("<omitted size=%d sha256=%s>", len(b), hex.EncodeToString(sum[:]))
}

func boundedList(in []string) []string {
	if len(in) > maxReportedEntries {
		in = in[:maxReportedEntries]
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, clip(s, maxReportedRunes))
	}
	return out
}
