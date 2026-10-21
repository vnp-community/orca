package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/prompts/outcome_review_draw"
	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// DrawingProfiles gives the drawing profile in force for a project: orca-default with the tenant and project layers over it.
type DrawingProfiles interface {
	Resolve(ctx context.Context, projectID string) (domain.ResolvedProfile, error)
}

// DefaultDrawingProfiles always answers orca-default. It is what runs until a profile store is wired.
type DefaultDrawingProfiles struct{}

func (DefaultDrawingProfiles) Resolve(context.Context, string) (domain.ResolvedProfile, error) {
	return domain.DefaultDrawingProfile(), nil
}

// ReviewDrawRunner draws the diagram models of a review with a read-only agent that uses the diagram-design skill. It draws only
// what the store does not already hold, so a review opened again, or the same model in another review, costs nothing.
type ReviewDrawRunner struct {
	Service   *OutcomeReviewService
	Runs      OutcomeReviewRunRepository
	Contexts  ReviewContextStore
	Artifacts *ReviewArtifactStore
	Profiles  DrawingProfiles
	Conns     AnalysisConnectionResolver
	Agent     AgentPromptRunner
	Caps      DevServerCapabilityReader
	Config    domain.OutcomeReviewConfig
	Settings  AnalysisSettings
	Probe     RepoStateProbe
	Observer  EngineObserver
	Spawn     func(func())
	Now       func() time.Time
	Nonce     func() string
	Log       *slog.Logger
}

func (r *ReviewDrawRunner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *ReviewDrawRunner) logger() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

func (r *ReviewDrawRunner) profiles() DrawingProfiles {
	if r.Profiles != nil {
		return r.Profiles
	}
	return DefaultDrawingProfiles{}
}

// StartDrawInput names what to draw. Force draws the selected diagrams again even when a drawing exists (regenerate).
type StartDrawInput struct {
	RequestID string
	// ReviewID is the review whose diagrams are drawn; empty means the newest one that is not superseded.
	ReviewID string
	// Sections limits the drawing to these sections; empty means every drawn section.
	Sections  []domain.ReviewSection
	Force     bool
	Reason    string
	Origin    domain.ReviewArtifactOrigin
	CreatedBy string
	// RunReason and ClientKey mark the run as one a person asked for and make a repeated request one run (see StartContextInput).
	RunReason string
	ClientKey string
}

// StartDrawResult says what happened to the diagrams the profile wants drawn.
type StartDrawResult struct {
	Run *ReviewRun
	// Cached is the number of diagrams that already had a drawing and were left alone.
	Cached int
	// Failed is the number of diagrams whose last attempt failed and were not tried again.
	Failed int
	// Pending is the number of diagrams handed to the agent.
	Pending int
	// Drawn is the number of diagrams Orca drew itself (the builtin engine): they are ready when the call returns.
	Drawn int
	// Skipped is no_context when the review has no diagram models yet, or draw_disabled when the deployment draws nothing.
	Skipped string
}

type drawJob struct {
	req       domain.Request
	review    StoredReview
	conn      AnalysisConnection
	profile   domain.ResolvedProfile
	skill     string
	pending   []ArtifactLookup
	reason    string
	origin    domain.ReviewArtifactOrigin
	createdBy string
}

// Start works out what is missing and, when something is, starts one draw run for it. Nothing missing means no agent call.
func (r *ReviewDrawRunner) Start(ctx context.Context, in StartDrawInput) (StartDrawResult, error) {
	req, err := r.Service.requireReadyRequest(ctx, in.RequestID)
	if err != nil {
		return StartDrawResult{}, err
	}
	if r.Config.DrawDisabled { // rollout stage 1: text only, and no agent run is ever spent on a drawing
		return StartDrawResult{Skipped: "draw_disabled"}, nil
	}
	review, err := r.reviewOf(ctx, in)
	if err != nil {
		return StartDrawResult{}, err
	}
	if review.ContextDigest == "" {
		return StartDrawResult{Skipped: "no_context"}, nil
	}
	rec, ok, err := r.Contexts.Get(ctx, req.ID, ReviewContextResult, review.ContextDigest, r.now())
	if err != nil {
		return StartDrawResult{}, err
	}
	var payload contextCachePayload
	if !ok || json.Unmarshal(rec.Payload, &payload) != nil {
		return StartDrawResult{Skipped: "no_context"}, nil // the models are gone with the cache; a new context run brings them back
	}
	profile, err := r.profiles().Resolve(ctx, req.ProjectID)
	if err != nil {
		return StartDrawResult{}, err
	}

	engine := r.Config.DrawEngineOrDefault()
	var conn AnalysisConnection
	var skill string
	if engine != domain.DrawEngineBuiltin {
		c, sk, aerr := r.agentRoute(ctx, req)
		switch {
		case aerr == nil:
			conn, skill = c, sk
		case engine == domain.DrawEngineAgent:
			return StartDrawResult{}, aerr
		default:
			engine = domain.DrawEngineBuiltin // auto: no agent can draw now, Orca draws it itself
		}
	}
	models := selectModels(payload.Context.Diagrams, in.Sections)
	if engine == domain.DrawEngineBuiltin {
		return r.drawBuiltin(ctx, req, models, profile, in)
	}
	lookups, err := r.Artifacts.Resolve(ctx, req.ID, models, profile, skill)
	if err != nil {
		return StartDrawResult{}, err
	}
	res := StartDrawResult{}
	var pending []ArtifactLookup
	for _, l := range lookups {
		switch {
		case in.Force:
			pending = append(pending, l)
		case l.Result == domain.CacheHit:
			res.Cached++
		case l.Result == domain.CacheFailed:
			res.Failed++
		default:
			pending = append(pending, l)
		}
	}
	res.Pending = len(pending)
	if len(pending) == 0 {
		return res, nil
	}

	run, err := startReviewRun(ctx, r.Runs, ReviewRun{ID: uuid.NewString(), ReviewID: review.ID, RequestID: req.ID, Kind: ReviewRunDraw, Status: ReviewRunRunning, Scope: string(review.Phase),
		Reason: in.RunReason, ClientKey: in.ClientKey, CreatedBy: in.CreatedBy},
		r.now(), time.Duration(r.Config.ContextTimeoutMS())*time.Millisecond*time.Duration(1+len(pending)/domain.MaxDrawingsPerRun)+2*time.Minute, r.logger())
	if err != nil {
		return StartDrawResult{}, err
	}
	res.Run = &run
	r.Service.Events.runProgress(ctx, run, run.Status, "")
	job := &drawJob{req: req, review: review, conn: conn, profile: profile, skill: skill, pending: pending, reason: in.Reason, origin: in.Origin, createdBy: in.CreatedBy}
	detached := tenant.Detach(ctx)
	spawn := r.Spawn
	if spawn == nil {
		spawn = func(f func()) { go f() }
	}
	spawn(func() { r.execute(detached, job, run) })
	return res, nil
}

// agentRoute says whether an agent can draw now: a dev server with a repository, the read-only mode and the skill. The error is
// the code a person would be told.
func (r *ReviewDrawRunner) agentRoute(ctx context.Context, req domain.Request) (AnalysisConnection, string, error) {
	conn, err := r.Conns.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		if errors.Is(err, ErrNoDevServer) {
			return AnalysisConnection{}, "", domain.ErrReviewNoDevServer("no dev server is connected for this project")
		}
		return AnalysisConnection{}, "", err
	}
	if conn.RepoPath == "" {
		return AnalysisConnection{}, "", domain.ErrReviewNoDevServer("the project's default repository has no folder path on its dev server")
	}
	caps := r.capability(ctx, conn)
	if gaps := domain.ReviewDrawGaps(caps); len(gaps) > 0 {
		for _, g := range gaps {
			if g.Code == domain.CodeReviewSkillMissing {
				return AnalysisConnection{}, "", domain.ErrReviewSkillMissing(domain.ReviewDrawSkill)
			}
		}
		return AnalysisConnection{}, "", domain.ErrReviewAgentTooOld(gaps[0].Message)
	}
	return conn, domain.ReviewDrawSkillVersion(caps), nil
}

// drawBuiltin lays the checked models out itself. Nothing runs on a dev server and nothing costs anything, so it is done before
// the call returns and there is no run to follow.
func (r *ReviewDrawRunner) drawBuiltin(ctx context.Context, req domain.Request, models []domain.DiagramModel, profile domain.ResolvedProfile, in StartDrawInput) (StartDrawResult, error) {
	lookups, err := r.Artifacts.Resolve(ctx, req.ID, models, profile, domain.BuiltinDrawSkillVersion)
	if err != nil {
		return StartDrawResult{}, err
	}
	res := StartDrawResult{}
	for _, l := range lookups {
		if !in.Force && l.Result == domain.CacheHit {
			res.Cached++
			continue
		}
		page, rerr := domain.RenderBuiltinDiagram(l.Model)
		var info domain.ArtifactInfo
		if rerr == nil {
			info, rerr = domain.CheckArtifact(page, l.Model)
		}
		if rerr != nil {
			if serr := r.Artifacts.StoreFailure(ctx, req.ID, l, appCodeOf(rerr, "REQUEST_REVIEW_ARTIFACT_UNSAFE"), rerr.Error(), in.CreatedBy); serr != nil {
				r.logger().ErrorContext(ctx, "outcome review: could not keep a failed builtin drawing", slog.Any("error", serr))
			}
			res.Failed++
			continue
		}
		if _, serr := r.Artifacts.Store(ctx, StoreInput{RequestID: req.ID, Lookup: l, Content: page, Info: info, Origin: in.Origin, Reason: in.Reason, CreatedBy: in.CreatedBy}); serr != nil {
			return res, serr
		}
		res.Drawn++
	}
	return res, nil
}

func (r *ReviewDrawRunner) reviewOf(ctx context.Context, in StartDrawInput) (StoredReview, error) {
	if in.ReviewID != "" {
		view, err := r.Service.Get(ctx, in.ReviewID)
		if err != nil {
			return StoredReview{}, err
		}
		if view.Stored.RequestID != in.RequestID {
			return StoredReview{}, domain.ErrReviewNotFound(in.ReviewID)
		}
		return view.Stored, nil
	}
	view, ok, err := r.Service.Latest(ctx, in.RequestID)
	if err != nil {
		return StoredReview{}, err
	}
	if !ok {
		return StoredReview{}, domain.ErrReviewNotFound(in.RequestID)
	}
	return view.Stored, nil
}

// selectModels keeps the models of the sections asked for, delta before design and in a fixed order, so the batches are stable.
func selectModels(models []domain.DiagramModel, sections []domain.ReviewSection) []domain.DiagramModel {
	want := map[domain.ReviewSection]bool{}
	for _, s := range sections {
		want[s] = true
	}
	var out []domain.DiagramModel
	for _, m := range models {
		if len(want) == 0 || want[m.Section] {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Variant != b.Variant {
			return a.Variant == domain.DiagramVariantDelta
		}
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		return a.Name < b.Name
	})
	return out
}

func (r *ReviewDrawRunner) capability(ctx context.Context, conn AnalysisConnection) domain.DevServerCapability {
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

type drawOutcome struct {
	stored  int
	failed  int
	code    string
	cost    domain.RunStats
	aborted bool
}

// execute draws the missing diagrams in batches, always ends the run, and never leaves the Request without its slot.
func (r *ReviewDrawRunner) execute(ctx context.Context, j *drawJob, run ReviewRun) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.Config.ContextTimeoutMS())*time.Millisecond*time.Duration(1+len(j.pending)/domain.MaxDrawingsPerRun)+time.Minute)
	defer cancel()
	var total drawOutcome
	finish := func(status ReviewRunStatus, code string) {
		if err := r.Runs.Finish(ctx, run.ID, status, code, costFigure(total.cost)); err != nil {
			r.logger().ErrorContext(ctx, "outcome review: could not end the draw run", slog.String("run_id", run.ID), slog.Any("error", err))
		}
		r.Service.Events.runFinished(ctx, run, status, code)
		observerOrNoop(r.Observer).RunFinished("review", string(ReviewRunDraw), string(status))
		ro := reviewObserverOf(r.Observer)
		ro.ReviewDrawn(total.stored, total.failed, code)
		observeRunEnd(ro, run, status, r.now(), total.cost.Usage, costFigure(total.cost))
		observerOrNoop(r.Observer).KnowledgeToolCalls(total.cost)
	}
	defer func() {
		if rec := recover(); rec != nil {
			r.logger().ErrorContext(ctx, "outcome review: draw run panicked", slog.String("run_id", run.ID), slog.Any("panic", rec))
			finish(ReviewRunFailed, "REQUEST_REVIEW_INTERNAL")
		}
	}()

	for start := 0; start < len(j.pending); start += domain.MaxDrawingsPerRun {
		end := start + domain.MaxDrawingsPerRun
		if end > len(j.pending) {
			end = len(j.pending)
		}
		out := r.drawBatch(ctx, j, j.pending[start:end])
		total.stored += out.stored
		total.failed += out.failed
		total.cost = total.cost.Merge(out.cost)
		if out.code != "" && total.code == "" {
			total.code = out.code
		}
		if out.aborted {
			break // the agent itself failed; the other batches would fail the same way
		}
	}
	switch {
	case total.stored == 0:
		finish(ReviewRunFailed, firstNonEmpty(total.code, "REQUEST_REVIEW_ARTIFACT_UNSAFE"))
	case total.failed > 0:
		finish(ReviewRunSucceeded, "REQUEST_REVIEW_PARTIAL")
	default:
		finish(ReviewRunSucceeded, "")
	}
}

// drawBatch asks the agent for up to MaxDrawingsPerRun drawings, checks each, keeps those that pass and asks once more, with the
// reasons, for those that do not. A diagram that fails twice is kept as a failed attempt and is not drawn again by itself.
func (r *ReviewDrawRunner) drawBatch(ctx context.Context, j *drawJob, batch []ArtifactLookup) drawOutcome {
	var out drawOutcome
	remaining := batch
	violations := map[int][]string{} // by position in batch
	index := map[string]int{}
	for i, l := range batch {
		index[drawIdentity(l.Model)] = i
	}
	failure := map[int]string{}
	for attempt := 0; attempt < 2 && len(remaining) > 0; attempt++ {
		nonce := r.newNonce()
		items := make([]outcome_review_draw.Item, 0, len(remaining))
		for _, l := range remaining {
			raw, err := modelForPrompt(l.Model)
			if err != nil {
				out.code = "REQUEST_REVIEW_INTERNAL"
				out.aborted = true
				return out
			}
			items = append(items, outcome_review_draw.Item{
				Section: string(l.Model.Section), Variant: l.Model.Variant, Name: l.Model.Name, Type: string(l.Model.Type),
				InputDigest: l.Model.InputDigest, ModelJSON: raw, Violations: violations[index[drawIdentity(l.Model)]],
			})
		}
		prompt, err := outcome_review_draw.Render(outcome_review_draw.Input{
			Items: items, Style: drawStyle(j.profile, remaining), Language: drawLanguage(j.profile), ResultNonce: nonce,
		})
		if err != nil {
			r.logger().WarnContext(ctx, "outcome review: draw prompt could not be built", slog.Any("error", err))
			out.code, out.aborted = "REQUEST_REVIEW_INTERNAL", true
			return out
		}
		call := AgentPromptInput{
			StepID: j.review.ID, Prompt: prompt, RepoPath: j.conn.RepoPath, RequestID: j.req.ID, ProjectID: j.req.ProjectID,
			TimeoutMS: r.Config.ContextTimeoutMS(), ReadOnlyEnforced: true, ResultNonce: nonce,
		}
		before, compared := snapshotOf(ctx, r.Probe, j.conn.WorktreeID, r.logger())
		res, err := r.Agent.ExecPrompt(ctx, j.conn, call)
		if err != nil {
			out.code, out.aborted = agentErrorCode(err), true
			return out
		}
		out.cost = out.cost.Merge(res.Stats)
		if code := checkReadonlyReviewRun(ctx, r.Probe, res, before, compared, j.conn.WorktreeID); code != "" {
			out.code, out.aborted = code, true
			return out
		}

		parsed, perr := parseDraw(res)
		if perr != nil {
			for i := range remaining {
				violations[index[drawIdentity(remaining[i].Model)]] = []string{perr.Error()}
			}
			continue
		}
		var still []ArtifactLookup
		byID := map[string]domain.DrawnArtifact{}
		for _, a := range parsed.Artifacts {
			byID[drawnIdentity(a)] = a
		}
		for _, l := range remaining {
			pos := index[drawIdentity(l.Model)]
			drawn, found := byID[drawIdentity(l.Model)]
			if !found {
				violations[pos] = []string{"there is no drawing for this diagram; copy section, variant, name and input_digest exactly"}
				failure[pos] = domain.CodeReviewResultInvalid
				still = append(still, l)
				continue
			}
			info, cerr := domain.CheckArtifact([]byte(drawn.HTML), l.Model)
			if cerr != nil {
				violations[pos] = []string{cerr.Error()}
				failure[pos] = appCodeOf(cerr, "REQUEST_REVIEW_ARTIFACT_UNSAFE")
				still = append(still, l)
				continue
			}
			if _, serr := r.Artifacts.Store(ctx, StoreInput{
				RequestID: j.req.ID, Lookup: l, Content: []byte(drawn.HTML), Info: info, Origin: j.origin, Reason: j.reason, CreatedBy: j.createdBy,
			}); serr != nil {
				if domainCode(serr) == "REQUEST_REVIEW_BUDGET_EXCEEDED" {
					out.code, out.aborted = domainCode(serr), true
					return out
				}
				return drawInternal(out)
			}
			out.stored++
		}
		remaining = still
	}
	for _, l := range remaining {
		pos := index[drawIdentity(l.Model)]
		reason := strings.Join(violations[pos], "; ")
		code := firstNonEmpty(failure[pos], domain.CodeReviewResultInvalid)
		if err := r.Artifacts.StoreFailure(ctx, j.req.ID, l, code, reason, j.createdBy); err != nil {
			r.logger().ErrorContext(ctx, "outcome review: could not keep a failed drawing", slog.Any("error", err))
		}
		out.failed++
		if out.code == "" {
			out.code = code
		}
	}
	return out
}

func drawInternal(o drawOutcome) drawOutcome {
	o.code, o.aborted = "REQUEST_REVIEW_INTERNAL", true
	return o
}

func (r *ReviewDrawRunner) newNonce() string {
	if r.Nonce != nil {
		return r.Nonce()
	}
	return randomNonce()
}

// parseDraw reads the result block, or the one JSON object in the output of an agent that sends no block. The error text is
// handed back to the agent, so it names the problem.
func parseDraw(res AgentPromptResult) (domain.DrawResult, error) {
	block := res.ResultBlock
	if len(block) == 0 {
		var err error
		if block, err = ExtractJSONObject(res.Stdout); err != nil {
			return domain.DrawResult{}, errors.New("the reply has no result block between ORCA_RESULT_BEGIN and ORCA_RESULT_END with the nonce; do not ask a question, draw")
		}
	}
	parsed, err := domain.ParseDrawResult(block)
	if err != nil {
		if v, ok := domain.AsContextViolations(err); ok {
			return domain.DrawResult{}, errors.New(strings.Join(v, "; "))
		}
		return domain.DrawResult{}, err
	}
	return parsed, nil
}

func drawIdentity(m domain.DiagramModel) string {
	return string(m.Section) + "|" + m.Variant + "|" + m.Name + "|" + m.InputDigest
}

func drawnIdentity(a domain.DrawnArtifact) string {
	return string(a.Section) + "|" + a.Variant + "|" + a.Name + "|" + a.InputDigest
}

// modelForPrompt is the model without its digest (the prompt carries that apart): the agent sees only what it has to draw.
func modelForPrompt(m domain.DiagramModel) (string, error) {
	m.InputDigest = ""
	raw, err := json.Marshal(m)
	return string(raw), err
}

func drawStyle(p domain.ResolvedProfile, items []ArtifactLookup) outcome_review_draw.Style {
	st := outcome_review_draw.Style{
		Detail: p.Style.Detail, Audience: p.Style.Audience, Size: p.Style.Size, Motion: p.Style.Motion, Sketchy: p.Style.Sketchy, Icons: p.Style.Icons,
		MaxNodes: domain.MaxDiagramNodes, Roles: p.Style.Roles,
	}
	for _, l := range items { // the smallest budget among the sections of the batch is the one that holds for all of them
		if n := p.Sections[l.Model.Section].MaxNodes; n > 0 && n < st.MaxNodes {
			st.MaxNodes = n
		}
	}
	return st
}

func drawLanguage(p domain.ResolvedProfile) output_language.Language {
	if p.Language == "vi" {
		return output_language.Vietnamese
	}
	return output_language.English
}

func snapshotOf(ctx context.Context, probe RepoStateProbe, worktreeID string, log *slog.Logger) (RepoSnapshot, bool) {
	if probe == nil || worktreeID == "" {
		return RepoSnapshot{}, false
	}
	snap, err := probe.Snapshot(ctx, worktreeID)
	if err != nil {
		if !errors.Is(err, ErrProbeUnavailable) {
			log.WarnContext(ctx, "outcome review: repository snapshot failed; comparison skipped", slog.Any("error", err))
		}
		return RepoSnapshot{}, false
	}
	return snap, true
}

func appCodeOf(err error, fallback string) string {
	if c := domainCode(err); c != "" {
		return c
	}
	return fallback
}
