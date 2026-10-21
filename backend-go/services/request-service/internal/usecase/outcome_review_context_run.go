package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewContextRunner starts and executes the context run of an Outcome Review: one read-only agent run that reads the knowledge
// graph and the SAD (and, after coding, the code) and returns the design and the diagram models. It never writes to the project:
// the run is enforced read-only, and what comes back is checked by domain.ValidateContext before anything is stored.
type ReviewContextRunner struct {
	Service  *OutcomeReviewService
	Runs     OutcomeReviewRunRepository
	Contexts ReviewContextStore
	Conns    AnalysisConnectionResolver
	Agent    AgentPromptRunner
	Caps     DevServerCapabilityReader
	Config   domain.OutcomeReviewConfig
	Settings AnalysisSettings
	// Probe and Files and Projects are optional; without them the run skips the repository comparison, assumes the knowledge
	// config is present, and shows no project name.
	Probe    RepoStateProbe
	Files    RepoFileStat
	Projects ProjectContextReader
	Observer EngineObserver
	// Draw may be nil; when set, the diagrams of a review are drawn as soon as its context is in, and again when a cached context is
	// applied, so a skill installed later is enough to get the drawings.
	Draw *ReviewDrawRunner
	// Spawn runs the background part. The default starts a goroutine; a test passes a function that runs it inline.
	Spawn func(func())
	Now   func() time.Time
	Nonce func() string
	Log   *slog.Logger
}

func (r *ReviewContextRunner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *ReviewContextRunner) logger() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

func (r *ReviewContextRunner) lock() ReviewDesignLock {
	return ReviewDesignLock{Store: r.Contexts, TTL: r.Config.ContextTTLOrDefault(), Now: r.Now}
}

// StartContextInput names what to run. Force ignores the cache and the locked design: it is what "regenerate" means.
type StartContextInput struct {
	RequestID     string
	Phase         domain.ReviewPhase
	Force         bool
	AllowCodeOnly bool
	CreatedBy     string
	// Regeneration: Reason marks the run as one a person asked for (it counts against the daily limit), ClientKey makes a repeated
	// request one run, and ForceDraw draws every diagram again afterwards instead of only those without a drawing.
	Reason    string
	ClientKey string
	ForceDraw bool
}

// StartContextResult says what happened: the review as it stands now, and either a run that was started, a cached context that
// was applied, or the reason nothing was started.
type StartContextResult struct {
	View    ReviewView
	Created bool
	Reused  bool
	Run     *ReviewRun
	// ContextCached is true when a checked result for the same inputs was applied and no agent was called.
	ContextCached bool
	// Skipped is "kg_unavailable" when the project has no knowledge graph and a code-only run was not asked for.
	Skipped string
	// Draw is what happened to the drawings of the review when the context was already there: the run that was started, or the code
	// of the error that kept one from starting (for instance the skill is missing). Nil when the context run is still going.
	Draw *StartDrawResult
	// DrawError is the code of that error.
	DrawError string
}

type contextPlan struct {
	mode          domain.ContextMode
	locked        *LockedDesign
	digest        string
	codeOnly      bool
	useKnowledge  bool
	req           domain.Request
	input         StartContextInput
	collected     Collected
	stored        StoredReview
	conn          AnalysisConnection
	profile       domain.DevServerCapability
	allowCodeOnly bool
	// stats is what the agent reported over the whole run, kept so the end of the run can report tokens.
	stats domain.RunStats
}

// contextCachePayload is a checked result as it is kept: the design digest it was made with travels along so applying it
// restores the review's link to the locked design.
type contextCachePayload struct {
	DesignDigest string                  `json:"designDigest"`
	Context      domain.ValidatedContext `json:"context"`
}

// Start builds the facts of the review (synchronously, at no AI cost), then applies a cached context result or starts a run.
func (r *ReviewContextRunner) Start(ctx context.Context, in StartContextInput) (StartContextResult, error) {
	gen, err := r.Service.Generate(ctx, GenerateInput{RequestID: in.RequestID, Phase: in.Phase, CreatedBy: in.CreatedBy})
	if err != nil {
		return StartContextResult{}, err
	}
	res := StartContextResult{View: gen.View, Created: gen.Created, Reused: gen.Reused}
	req, err := r.Service.requireReadyRequest(ctx, in.RequestID)
	if err != nil {
		return res, err
	}
	plan := &contextPlan{mode: domain.ContextMode(gen.View.Stored.Phase), req: req, input: in, collected: gen.Collected, stored: gen.View.Stored, allowCodeOnly: in.AllowCodeOnly}
	if plan.mode == domain.ContextModeDelivered && !in.Force {
		if l, ok, lerr := r.lock().Load(ctx, req.ID); lerr != nil {
			return res, lerr
		} else if ok {
			plan.locked = &l
		}
	}

	if !in.Force {
		if applied, ok, cerr := r.applyCached(ctx, plan, false); cerr != nil {
			return res, cerr
		} else if ok {
			res.View, res.ContextCached = applied, true
			res.Draw, res.DrawError = r.drawAfter(ctx, req.ID, in)
			return res, nil
		}
	}

	conn, err := r.Conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return res, domain.ErrReviewNoDevServer("no dev server is connected for this project")
		}
		return res, err
	}
	if conn.RepoPath == "" {
		return res, domain.ErrReviewNoDevServer("the project's default repository has no folder path on its dev server")
	}
	plan.conn, plan.profile = conn, r.capability(ctx, conn)
	skipped, err := r.decide(ctx, plan)
	if err != nil || skipped != "" {
		res.Skipped = skipped
		return res, err
	}
	if plan.codeOnly && !in.Force { // the code-only variant has its own cache entry
		if applied, ok, cerr := r.applyCached(ctx, plan, true); cerr != nil {
			return res, cerr
		} else if ok {
			res.View, res.ContextCached = applied, true
			res.Draw, res.DrawError = r.drawAfter(ctx, req.ID, in)
			return res, nil
		}
	}

	run, err := r.startRun(ctx, plan)
	if err != nil {
		return res, err
	}
	res.Run = &run
	r.Service.Events.runProgress(ctx, run, run.Status, "")
	r.audit(ctx, plan, run)
	detached := tenant.Detach(ctx)
	spawn := r.Spawn
	if spawn == nil {
		spawn = func(f func()) { go f() }
	}
	spawn(func() { r.execute(detached, plan, run, in.Force) })
	return res, nil
}

// startRun takes the one slot a Request has for a context run. A run that has been active for longer than any run can last was
// cut off by a restart: it is ended as interrupted so it does not block the Request for good.
func (r *ReviewContextRunner) startRun(ctx context.Context, p *contextPlan) (ReviewRun, error) {
	return startReviewRun(ctx, r.Runs, ReviewRun{ID: uuid.NewString(), ReviewID: p.stored.ID, RequestID: p.req.ID, Kind: ReviewRunContext, Status: ReviewRunRunning, Scope: string(p.mode),
		Reason: p.input.Reason, ClientKey: p.input.ClientKey, CreatedBy: p.input.CreatedBy},
		r.now(), time.Duration(r.Config.ContextTimeoutMS())*time.Millisecond+2*time.Minute, r.logger())
}

// startReviewRun takes the slot of a kind of run and ends a run that outlived maxAge, which only a restart can cause.
func startReviewRun(ctx context.Context, runs OutcomeReviewRunRepository, in ReviewRun, now time.Time, maxAge time.Duration, log *slog.Logger) (ReviewRun, error) {
	if in.CostEstimate == "" {
		in.CostEstimate = predictedCost(ctx, runs, in.ReviewID, in.Kind)
	}
	run, err := runs.Start(ctx, in)
	if err == nil || domainCode(err) != domain.CodeReviewRunInProgress {
		return run, err
	}
	cur, ok, aerr := runs.ActiveForRequest(ctx, in.RequestID, in.Kind)
	if aerr != nil || !ok || now.Sub(cur.StartedAt) <= maxAge {
		return ReviewRun{}, err
	}
	log.WarnContext(ctx, "outcome review: ending a run that outlived its limit", slog.String("run_id", cur.ID), slog.String("kind", string(in.Kind)))
	if ferr := runs.Finish(ctx, cur.ID, ReviewRunFailed, "REQUEST_REVIEW_INTERRUPTED", ""); ferr != nil {
		return ReviewRun{}, err
	}
	return runs.Start(ctx, in)
}

// decide settles whether the run reads the graph, reads code only, or does not start. It spends nothing.
func (r *ReviewContextRunner) decide(ctx context.Context, p *contextPlan) (skipped string, err error) {
	if !p.profile.HasFeature(domain.FeatureReadonly) {
		return "", domain.ErrReviewAgentTooOld("it cannot enforce read-only mode")
	}
	if p.locked != nil { // the design is known: the delivered run reads code only and needs no graph
		return "", nil
	}
	if r.knowledgeConfigAbsent(ctx, p.conn) {
		if p.mode == domain.ContextModeDelivered && p.allowCodeOnly {
			p.codeOnly = true
			return "", nil
		}
		return "kg_unavailable", nil
	}
	if domain.ReviewSkillMissing(p.profile) {
		return "", domain.ErrReviewSkillMissing(domain.ReviewContextSkill)
	}
	if gaps := domain.ReviewContextGaps(p.profile); len(gaps) > 0 {
		return "", domain.ErrReviewAgentTooOld(gaps[0].Message)
	}
	p.useKnowledge = true
	return "", nil
}

// knowledgeConfigAbsent is true only when the file system says .tdd-agent/config.toml is not there; a check that cannot be made
// does not stop a run, which then finds out when the agent starts.
func (r *ReviewContextRunner) knowledgeConfigAbsent(ctx context.Context, conn AnalysisConnection) bool {
	if r.Files == nil || conn.WorktreeID == "" {
		return false
	}
	info, err := r.Files.Stat(ctx, Workspace{WorktreeID: conn.WorktreeID, Path: conn.RepoPath}, knowledgeConfigPath)
	return err == nil && (!info.Exists || info.IsDir)
}

func (r *ReviewContextRunner) capability(ctx context.Context, conn AnalysisConnection) domain.DevServerCapability {
	if r.Caps == nil {
		return domain.DevServerCapability{}
	}
	got, err := r.Caps.Get(ctx, DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}, false)
	if err != nil {
		if !errors.Is(err, ErrCapabilityNotAvailable) {
			r.logger().WarnContext(ctx, "outcome review: capability profile unreadable", slog.Any("error", err))
		}
		return domain.DevServerCapability{}
	}
	return got
}

// digestFor is the key of a checked result. A code-only run reads other things than a graph run, so it is keyed apart.
func (r *ReviewContextRunner) digestFor(p *contextPlan, codeOnly bool) (string, error) {
	lockedDigest := ""
	if p.locked != nil {
		lockedDigest = p.locked.Digest
	}
	version := ReviewContextPromptVersion
	if codeOnly {
		version += "|code-only"
	}
	// Why: the delivered wording changed (uncommitted worktree work) without touching the design lock, so only its results are re-keyed.
	if p.mode == domain.ContextModeDelivered {
		version += "|delivered-3"
	}
	return domain.ContextDigest(p.mode, p.stored.InputsDigest, lockedDigest, version)
}

// applyCached looks for a checked result for the same inputs and, if there is one, applies it to the review.
func (r *ReviewContextRunner) applyCached(ctx context.Context, p *contextPlan, codeOnly bool) (ReviewView, bool, error) {
	digest, err := r.digestFor(p, codeOnly)
	if err != nil {
		return ReviewView{}, false, err
	}
	p.digest = digest
	rec, ok, err := r.Contexts.Get(ctx, p.req.ID, ReviewContextResult, digest, r.now())
	if err != nil {
		return ReviewView{}, false, err
	}
	var payload contextCachePayload
	if ok && json.Unmarshal(rec.Payload, &payload) != nil {
		ok = false // an unreadable record is a miss, and the next run replaces it
	}
	reviewObserverOf(r.Observer).ReviewContextLookup(ok)
	if !ok {
		return ReviewView{}, false, nil
	}
	if p.stored.ContextDigest == digest {
		view, derr := decodeReview(p.stored)
		return view, true, derr
	}
	view, err := r.Service.ApplyContext(ctx, p.req, p.collected, p.stored, payload.Context, digest, payload.DesignDigest)
	if err != nil {
		return ReviewView{}, false, err
	}
	return view, true, nil
}

func (r *ReviewContextRunner) audit(ctx context.Context, p *contextPlan, run ReviewRun) {
	logReviewAudit(ctx, r.logger(), domain.AuditReviewContext, p.req.ID, p.stored.ID, p.stored.InputsDigest)
	if r.Service.Audit == nil {
		return
	}
	_ = r.Service.Audit.Record(ctx, AuditEvent{
		Action: domain.AuditReviewContext, TargetType: "request", TargetID: p.req.ID, RequestID: p.req.ID, Outcome: "allowed",
		Metadata: map[string]any{"review_id": p.stored.ID, "run_id": run.ID, "mode": string(p.mode), "knowledge": p.useKnowledge, "code_only": p.codeOnly},
	})
}

// execute is the background half. It always ends the run, whatever happens.
func (r *ReviewContextRunner) execute(ctx context.Context, p *contextPlan, run ReviewRun, force bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.Config.ContextTimeoutMS())*time.Millisecond+time.Minute)
	defer cancel()
	cost := ""
	finish := func(status ReviewRunStatus, code string) {
		if err := r.Runs.Finish(ctx, run.ID, status, code, cost); err != nil {
			r.logger().ErrorContext(ctx, "outcome review: could not end the run", slog.String("run_id", run.ID), slog.Any("error", err))
		}
		r.Service.Events.runFinished(ctx, run, status, code)
		observerOrNoop(r.Observer).RunFinished("review", string(ReviewRunContext), string(status))
		observeRunEnd(reviewObserverOf(r.Observer), run, status, r.now(), p.stats.Usage, cost)
	}
	defer func() {
		if rec := recover(); rec != nil {
			r.logger().ErrorContext(ctx, "outcome review: context run panicked", slog.String("run_id", run.ID), slog.Any("panic", rec))
			finish(ReviewRunFailed, "REQUEST_REVIEW_INTERNAL")
			_ = r.Service.MarkContextFailed(ctx, p.stored.ID)
		}
	}()

	vc, designDigest, usage, code := r.runAgent(ctx, p)
	cost = usage
	if code != "" {
		finish(ReviewRunFailed, code)
		if code == domain.CodeReviewKnowledgeRequired {
			_ = r.Service.MarkContextWarning(ctx, p.stored.ID, "kg_unavailable")
		} else {
			_ = r.Service.MarkContextFailed(ctx, p.stored.ID)
		}
		return
	}
	digest, err := r.digestFor(p, p.codeOnly)
	if err != nil {
		finish(ReviewRunFailed, "REQUEST_REVIEW_INTERNAL")
		return
	}
	raw, err := json.Marshal(contextCachePayload{DesignDigest: designDigest, Context: vc})
	if err == nil {
		_, err = r.Contexts.Put(ctx, ReviewContextRecord{
			ID: uuid.NewString(), RequestID: p.req.ID, Kind: ReviewContextResult, Digest: digest, PromptVersion: ReviewContextPromptVersion,
			Payload: raw, ExpiresAt: r.now().Add(r.Config.ContextTTLOrDefault()),
		})
	}
	if err != nil {
		r.logger().ErrorContext(ctx, "outcome review: could not keep the context result", slog.String("run_id", run.ID), slog.Any("error", err))
	}
	if _, err := r.Service.ApplyContext(ctx, p.req, p.collected, p.stored, vc, digest, designDigest); err != nil {
		r.logger().ErrorContext(ctx, "outcome review: could not apply the context result", slog.String("run_id", run.ID), slog.Any("error", err))
		finish(ReviewRunFailed, "REQUEST_REVIEW_INTERNAL")
		return
	}
	finish(ReviewRunSucceeded, "")
	r.drawAfter(ctx, p.req.ID, p.input)
}

// drawAfter hands the review to the draw runner. A drawing that cannot start (no skill, no dev server) leaves the diagrams missing,
// which the review shows; it never fails the context run that came before.
func (r *ReviewContextRunner) drawAfter(ctx context.Context, requestID string, in StartContextInput) (*StartDrawResult, string) {
	if r.Draw == nil {
		return nil, ""
	}
	draw := StartDrawInput{RequestID: requestID, CreatedBy: in.CreatedBy, Origin: domain.ArtifactOriginAuto, Force: in.ForceDraw}
	if in.ForceDraw {
		draw.Origin, draw.Reason = domain.ArtifactOriginRegenerated, in.Reason
	}
	res, err := r.Draw.Start(ctx, draw)
	if err != nil {
		code := domainCode(err)
		r.logger().InfoContext(ctx, "outcome review: the drawings did not start", slog.String("request_id", requestID), slog.String("code", code))
		return nil, code
	}
	return &res, ""
}

// runAgent calls the agent, giving it one retry with the violations of a result that broke the contract. It returns a failure
// code, or the checked context with the digest of the design it used and the cost figure.
func (r *ReviewContextRunner) runAgent(ctx context.Context, p *contextPlan) (domain.ValidatedContext, string, string, string) {
	projectName, repoURL := projectFacts(ctx, r.Projects, p.req.ProjectID)
	var stats domain.RunStats
	defer func() { p.stats = stats }()
	useKnowledge := p.useKnowledge
	var violations []string
	for attempt := 0; attempt < 2; attempt++ {
		nonce := r.newNonce()
		var locked *domain.DesignModel
		if p.locked != nil {
			locked = &p.locked.Design
		}
		prompt, err := RenderReviewContextPrompt(ReviewPromptInput{
			Mode: p.mode, Request: p.req, ProjectName: projectName, RepoURL: repoURL, UseKnowledge: useKnowledge, Locked: locked,
			Facts: p.collected.Facts, MaxFiles: r.Config.ContextMaxFilesOrDefault(), Nonce: nonce, Violations: violations,
			Language: r.Settings.OutputLanguageFor(p.req),
		})
		if err != nil {
			r.logger().WarnContext(ctx, "outcome review: prompt could not be built", slog.Any("error", err))
			return domain.ValidatedContext{}, "", costFigure(stats), "REQUEST_REVIEW_INTERNAL"
		}
		call := AgentPromptInput{
			StepID: p.stored.ID, Prompt: prompt, RepoPath: p.conn.RepoPath, RequestID: p.req.ID, ProjectID: p.req.ProjectID,
			TimeoutMS: r.Config.ContextTimeoutMS(), ReadOnlyEnforced: true, ResultNonce: nonce,
		}
		if useKnowledge {
			call.KnowledgeRoot = p.conn.RepoPath
		}
		before, repoCheck := r.snapshot(ctx, p.conn.WorktreeID)
		res, err := r.Agent.ExecPrompt(ctx, p.conn, call)
		if err != nil {
			if errors.Is(err, ErrAgentKnowledgeHubUnavailable) && useKnowledge {
				if p.mode == domain.ContextModeDelivered && p.allowCodeOnly {
					useKnowledge, p.codeOnly = false, true // the run did not start, so asking again without the graph costs nothing
					attempt--
					continue
				}
				return domain.ValidatedContext{}, "", costFigure(stats), domain.CodeReviewKnowledgeRequired
			}
			return domain.ValidatedContext{}, "", costFigure(stats), agentErrorCode(err)
		}
		stats = stats.Merge(res.Stats)
		observerOrNoop(r.Observer).KnowledgeToolCalls(res.Stats)
		if code := r.checkRun(ctx, res, before, repoCheck, p.conn.WorktreeID); code != "" {
			return domain.ValidatedContext{}, "", costFigure(stats), code
		}

		vc, verr := r.validate(ctx, p, res)
		if verr == nil {
			designDigest, derr := r.keepDesign(ctx, p, vc)
			if derr != nil {
				return domain.ValidatedContext{}, "", costFigure(stats), "REQUEST_REVIEW_INTERNAL"
			}
			return vc, designDigest, costFigure(stats), ""
		}
		if v, ok := domain.AsContextViolations(verr); ok {
			violations = v
			continue
		}
		return domain.ValidatedContext{}, "", costFigure(stats), "REQUEST_REVIEW_INTERNAL"
	}
	return domain.ValidatedContext{}, "", costFigure(stats), domain.CodeReviewResultInvalid
}

func (r *ReviewContextRunner) newNonce() string {
	if r.Nonce != nil {
		return r.Nonce()
	}
	return randomNonce()
}

// keepDesign locks the design the run extracted, or keeps pointing at the lock it used.
func (r *ReviewContextRunner) keepDesign(ctx context.Context, p *contextPlan, vc domain.ValidatedContext) (string, error) {
	if p.locked != nil {
		return p.locked.Digest, nil
	}
	if !vc.Design.Available {
		return "", nil
	}
	l, err := r.lock().Save(ctx, p.req.ID, vc.Design)
	return l.Digest, err
}

// validate reads the result block (or, from an agent that sends none, the one JSON object in its output) and checks it.
func (r *ReviewContextRunner) validate(ctx context.Context, p *contextPlan, res AgentPromptResult) (domain.ValidatedContext, error) {
	block := res.ResultBlock
	if len(block) == 0 {
		var err error
		if block, err = ExtractJSONObject(res.Stdout); err != nil {
			return domain.ValidatedContext{}, domain.ContextViolations{"the reply has no result block between ORCA_RESULT_BEGIN and ORCA_RESULT_END with the nonce"}
		}
	}
	raw, err := domain.ParseContextResult(block)
	if err != nil {
		return domain.ValidatedContext{}, err
	}
	var lockedDesign *domain.DesignModel
	if p.locked != nil {
		lockedDesign = &p.locked.Design
	}
	return domain.ValidateContext(raw, domain.ContextValidationInput{
		Mode: p.mode, Evidence: r.evidenceContext(ctx, p), Redact: r.Service.Redact, LockedDesign: lockedDesign,
	})
}

// evidenceContext is what the evidence of this Request can be checked against: its own diff, checks and tasks.
func (r *ReviewContextRunner) evidenceContext(ctx context.Context, p *contextPlan) domain.EvidenceContext {
	f := p.collected.Facts
	ev := domain.EvidenceContext{ChangedFiles: map[string]bool{}, CheckIDs: map[string]bool{}, TaskIDs: map[string]bool{}}
	var paths []string
	for _, c := range f.Changed {
		pth := strings.TrimPrefix(strings.ReplaceAll(c.Path, "\\", "/"), "./")
		ev.ChangedFiles[pth] = true
		paths = append(paths, pth)
	}
	ev.ContractChanged = domain.ContractPathChanged(paths)
	for _, row := range f.Requests.Coverage {
		if row.CheckID != "" {
			ev.CheckIDs[row.CheckID] = true
		}
	}
	for id := range f.Requests.Tasks {
		ev.TaskIDs[id] = true
	}
	ev.BaseHasFile = r.baseHasFile(ctx, p.conn)
	return ev
}

// baseHasFile asks the project's checkout whether a path exists; the answers are kept for the run, and a check that fails is a no.
func (r *ReviewContextRunner) baseHasFile(ctx context.Context, conn AnalysisConnection) func(string) bool {
	if r.Files == nil || conn.WorktreeID == "" {
		return nil
	}
	seen := map[string]bool{}
	const maxLookups = 200
	return func(path string) bool {
		if v, ok := seen[path]; ok {
			return v
		}
		if len(seen) >= maxLookups {
			return false
		}
		info, err := r.Files.Stat(ctx, Workspace{WorktreeID: conn.WorktreeID, Path: conn.RepoPath}, path)
		seen[path] = err == nil && info.Exists && !info.IsDir
		return seen[path]
	}
}

func (r *ReviewContextRunner) snapshot(ctx context.Context, worktreeID string) (RepoSnapshot, bool) {
	if r.Probe == nil || worktreeID == "" {
		return RepoSnapshot{}, false
	}
	snap, err := r.Probe.Snapshot(ctx, worktreeID)
	if err != nil {
		if !errors.Is(err, ErrProbeUnavailable) {
			r.logger().WarnContext(ctx, "outcome review: repository snapshot failed; comparison skipped", slog.Any("error", err))
		}
		return RepoSnapshot{}, false
	}
	return snap, true
}

// checkRun turns an agent result into a failure code when the run itself went wrong or the repository changed. A context run is
// enforced read-only, so any change is a violation and the result is thrown away.
func (r *ReviewContextRunner) checkRun(ctx context.Context, res AgentPromptResult, before RepoSnapshot, compared bool, worktreeID string) string {
	return checkReadonlyReviewRun(ctx, r.Probe, res, before, compared, worktreeID)
}

// checkReadonlyReviewRun is the check both review runs share: neither may change the repository.
func checkReadonlyReviewRun(ctx context.Context, probe RepoStateProbe, res AgentPromptResult, before RepoSnapshot, compared bool, worktreeID string) string {
	switch {
	case res.TimedOut:
		return "REQUEST_REVIEW_TIMEOUT"
	case res.ExitCode != 0:
		return "REQUEST_REVIEW_AGENT_FAILED"
	}
	for _, w := range res.Warnings {
		if w == "READONLY_VIOLATION" {
			return "REQUEST_REVIEW_REPO_MODIFIED"
		}
	}
	if res.AppliedAccessMode != "readonly" {
		return "REQUEST_REVIEW_READONLY_NOT_ACTIVE"
	}
	if res.HeadMoved || res.ChangedFiles > 0 {
		return "REQUEST_REVIEW_REPO_MODIFIED"
	}
	if compared && probe != nil {
		if after, err := probe.Snapshot(ctx, worktreeID); err == nil && !before.Equal(after) {
			return "REQUEST_REVIEW_REPO_MODIFIED"
		}
	}
	return ""
}

func agentErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrAgentReadonlyUnsupported):
		return domain.CodeReviewAgentTooOld
	case errors.Is(err, ErrNoDevServer):
		return "REQUEST_REVIEW_NO_DEV_SERVER"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrClassifierTimeout):
		return "REQUEST_REVIEW_TIMEOUT"
	}
	return "REQUEST_REVIEW_AGENT_FAILED"
}

// costFigure is the cost claude reported for the run, or empty when it did not say; a missing figure is not zero.
func costFigure(s domain.RunStats) string {
	if s.Usage == nil || s.Usage.CostUSD == nil {
		return ""
	}
	return fmt.Sprintf("%.4f USD", *s.Usage.CostUSD)
}

// ContextEstimate says what a context run would do now, without running anything: whether a checked result is kept, whether the
// design is locked, and whether the dev server could run it.
type ContextEstimate struct {
	Cached       bool
	DesignLocked bool
	Runnable     bool
	Gaps         []string
}

// EstimateContext reads the cache, the design lock and the dev server's capability report. It spends no agent run.
func (r *ReviewContextRunner) EstimateContext(ctx context.Context, requestID string, est ReviewEstimate) (ContextEstimate, error) {
	out := ContextEstimate{}
	req, err := r.Service.requireReadyRequest(ctx, requestID)
	if err != nil {
		return out, err
	}
	p := &contextPlan{mode: domain.ContextMode(est.Phase), req: req, stored: StoredReview{InputsDigest: est.CurrentInputsDigest}}
	if p.mode == domain.ContextModeDelivered {
		if l, ok, lerr := r.lock().Load(ctx, req.ID); lerr != nil {
			return out, lerr
		} else if ok {
			p.locked, out.DesignLocked = &l, true
		}
	}
	digest, err := r.digestFor(p, false)
	if err != nil {
		return out, err
	}
	if _, ok, gerr := r.Contexts.Get(ctx, req.ID, ReviewContextResult, digest, r.now()); gerr != nil {
		return out, gerr
	} else if ok {
		out.Cached = true
	}

	conn, err := r.Conns.ResolveForProject(ctx, req.ProjectID)
	switch {
	case errors.Is(err, ErrNoDevServer):
		out.Gaps = append(out.Gaps, "no_dev_server")
		return out, nil
	case err != nil:
		return out, err
	case conn.RepoPath == "":
		out.Gaps = append(out.Gaps, "no_repo_path")
		return out, nil
	}
	p.conn, p.profile = conn, r.capability(ctx, conn)
	if !p.profile.HasFeature(domain.FeatureReadonly) {
		out.Gaps = append(out.Gaps, domain.CodeReviewAgentTooOld)
	}
	if p.locked == nil {
		if r.knowledgeConfigAbsent(ctx, conn) {
			out.Gaps = append(out.Gaps, "kg_unavailable")
		}
		for _, g := range domain.ReviewContextGaps(p.profile) {
			out.Gaps = append(out.Gaps, g.Code)
		}
	}
	out.Gaps = dedupeSortedStrings(out.Gaps)
	out.Runnable = len(out.Gaps) == 0
	return out, nil
}

// ListRuns returns the runs of the newest review of the Request that is not superseded, or of the review named. A review that
// belongs to another Request is not found.
func (r *ReviewContextRunner) ListRuns(ctx context.Context, requestID, reviewID string) ([]ReviewRun, error) {
	if reviewID == "" {
		view, ok, err := r.Service.Latest(ctx, requestID)
		if err != nil || !ok {
			return nil, err
		}
		reviewID = view.Stored.ID
	} else {
		view, err := r.Service.Get(ctx, reviewID)
		if err != nil {
			return nil, err
		}
		if view.Stored.RequestID != requestID {
			return nil, domain.ErrReviewNotFound(reviewID)
		}
	}
	return r.Runs.ListByReview(ctx, reviewID)
}
