package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewRegenerator makes a review again because a person asked, with a reason, within a limit, and (when it is expensive) after a
// confirmation. It never runs by itself. What it replaces is kept: an earlier drawing can be restored for nothing.
type ReviewRegenerator struct {
	Service   *OutcomeReviewService
	Context   *ReviewContextRunner
	Draw      *ReviewDrawRunner
	Runs      OutcomeReviewRunRepository
	Artifacts *ReviewArtifactStore
	Contexts  ReviewContextStore
	Profiles  DrawingProfiles
	Config    domain.OutcomeReviewConfig
	Now       func() time.Time
}

func (r *ReviewRegenerator) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// RegenerateInput is one request to regenerate.
type RegenerateInput struct {
	RequestID string
	domain.RegenRequest
	ConfirmCost     bool
	ClientRequestID string
	// ExpectedVersion is the version of the review the person was looking at; 0 skips the check.
	ExpectedVersion int64
	By              string
}

// RegenerateResult says which runs were started. Replayed is true when the same client request was answered before.
type RegenerateResult struct {
	ContextRun *ReviewRun
	DrawRun    *ReviewRun
	Replayed   bool
	Estimate   RegenEstimate
	// Skipped is kg_unavailable when a context run was asked for and the project has no knowledge graph.
	Skipped string
}

// RegenEstimate is what a regeneration is expected to do and cost. Nothing in it is invented: when no earlier run reported a cost,
// CostKnown is false and there is no amount.
type RegenEstimate struct {
	AgentRuns       int
	DiagramsToDraw  int
	CostKnown       bool
	CostUSD         float64
	LastContextCost string
	LastDrawCost    string
	ConfirmRequired bool
	// DrawingsDependOnResult is true for a context regeneration: how many diagrams change is known only after the run.
	DrawingsDependOnResult bool
}

// Estimate works out the plan without spending anything.
func (r *ReviewRegenerator) Estimate(ctx context.Context, requestID string, req domain.RegenRequest) (RegenEstimate, error) {
	if err := req.Validate(); err != nil {
		return RegenEstimate{}, err
	}
	view, ok, err := r.Service.Latest(ctx, requestID)
	if err != nil {
		return RegenEstimate{}, err
	}
	if !ok {
		return RegenEstimate{}, domain.ErrReviewNotFound(requestID)
	}
	return r.estimateFor(ctx, view, req)
}

func (r *ReviewRegenerator) estimateFor(ctx context.Context, view ReviewView, req domain.RegenRequest) (RegenEstimate, error) {
	est := RegenEstimate{DrawingsDependOnResult: req.Scope == domain.RegenContext}
	profile := domain.DefaultDrawingProfile()
	if r.Profiles != nil {
		if rq, err := r.Service.Requests.Get(ctx, view.Stored.RequestID); err == nil {
			if p, perr := r.Profiles.Resolve(ctx, rq.ProjectID); perr == nil {
				profile = p
			}
		}
	}
	if req.Scope.RunsContext() {
		est.AgentRuns++
	}
	if req.Scope.Draws() && !r.Service.Config.DrawDisabled {
		est.DiagramsToDraw = countDrawable(view.Review.Diagrams, profile, req.Sections)
		est.AgentRuns += (est.DiagramsToDraw + domain.MaxDrawingsPerRun - 1) / domain.MaxDrawingsPerRun
	}
	runs, err := r.Runs.ListByReview(ctx, view.Stored.ID)
	if err != nil {
		return est, err
	}
	var ctxCost, drawCost *float64
	for _, run := range runs { // newest first
		if run.Status != ReviewRunSucceeded {
			continue
		}
		c, ok := parseCostUSD(run.CostActual)
		if !ok {
			continue
		}
		switch {
		case run.Kind == ReviewRunContext && ctxCost == nil:
			ctxCost, est.LastContextCost = &c, run.CostActual
		case run.Kind == ReviewRunDraw && drawCost == nil:
			drawCost, est.LastDrawCost = &c, run.CostActual
		}
	}
	var total float64
	known := true
	if req.Scope.RunsContext() {
		if ctxCost == nil {
			known = false
		} else {
			total += *ctxCost
		}
	}
	if req.Scope.Draws() {
		if drawCost == nil {
			known = false
		} else {
			total += *drawCost * float64((est.DiagramsToDraw+domain.MaxDrawingsPerRun-1)/domain.MaxDrawingsPerRun)
		}
	}
	if known && (req.Scope.RunsContext() || req.Scope.Draws()) {
		est.CostKnown, est.CostUSD = true, total
	}
	est.ConfirmRequired = est.CostKnown && r.Config.ConfirmCostUSD > 0 && est.CostUSD > r.Config.ConfirmCostUSD
	return est, nil
}

func countDrawable(refs []domain.DiagramRef, profile domain.ResolvedProfile, sections []domain.ReviewSection) int {
	want := map[domain.ReviewSection]bool{}
	for _, s := range sections {
		want[s] = true
	}
	n := 0
	for _, ref := range refs {
		if profile.Draws(ref.Section, ref.Variant) && (len(want) == 0 || want[ref.Section]) {
			n++
		}
	}
	return n
}

// parseCostUSD reads "0.4200 USD" as the context and draw runs write it; anything else is "not reported".
func parseCostUSD(s string) (float64, bool) {
	f, ok := strings.CutSuffix(strings.TrimSpace(s), " USD")
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(f, 64)
	return v, err == nil && v >= 0
}

// Regenerate checks the request against the limits and starts the runs it needs.
func (r *ReviewRegenerator) Regenerate(ctx context.Context, in RegenerateInput) (RegenerateResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return RegenerateResult{}, domain.ErrRequestTenantRequired()
	}
	if err := in.RegenRequest.Validate(); err != nil {
		return RegenerateResult{}, err
	}
	if in.ClientRequestID != "" {
		if run, ok, err := r.Runs.FindByClientKey(ctx, in.RequestID, in.ClientRequestID); err != nil {
			return RegenerateResult{}, err
		} else if ok {
			return replayed(run), nil
		}
	}
	view, ok, err := r.Service.Latest(ctx, in.RequestID)
	if err != nil {
		return RegenerateResult{}, err
	}
	if !ok {
		return RegenerateResult{}, domain.ErrReviewNotFound(in.RequestID)
	}
	if in.ExpectedVersion != 0 && view.Stored.Version != in.ExpectedVersion {
		return RegenerateResult{}, domain.ErrReviewStaleRevision()
	}
	used, err := r.Runs.CountRegenerations(ctx, in.RequestID, r.now().Add(-24*time.Hour))
	if err != nil {
		return RegenerateResult{}, err
	}
	active, err := r.Runs.CountActive(ctx)
	if err != nil {
		return RegenerateResult{}, err
	}
	if err := (domain.RegenBudget{Limit: r.Config.RegenPerDayOrDefault(), UsedToday: used, ActiveInTenant: active, MaxConcurrent: domain.MaxConcurrentReviewRuns}).Check(); err != nil {
		return RegenerateResult{}, err
	}
	est, err := r.estimateFor(ctx, view, in.RegenRequest)
	if err != nil {
		return RegenerateResult{}, err
	}
	if est.ConfirmRequired && !in.ConfirmCost {
		return RegenerateResult{Estimate: est}, domain.ErrReviewConfirmCost(fmt.Sprintf("%.2f USD", est.CostUSD))
	}

	out, err := r.start(ctx, in, view)
	out.Estimate = est
	if err != nil {
		if domainCode(err) == "REQUEST_REVIEW_DUPLICATE_REQUEST" { // two copies of one request raced; the first one's run is the answer
			if run, found, ferr := r.Runs.FindByClientKey(ctx, in.RequestID, in.ClientRequestID); ferr == nil && found {
				return replayed(run), nil
			}
		}
		return out, err
	}
	r.audit(ctx, in, view)
	return out, nil
}

func replayed(run ReviewRun) RegenerateResult {
	out := RegenerateResult{Replayed: true}
	if run.Kind == ReviewRunDraw {
		out.DrawRun = &run
	} else {
		out.ContextRun = &run
	}
	return out
}

func (r *ReviewRegenerator) start(ctx context.Context, in RegenerateInput, view ReviewView) (RegenerateResult, error) {
	reason := string(in.Reason)
	out := RegenerateResult{}
	switch in.Scope {
	case domain.RegenArtifact:
		if r.Draw == nil {
			return out, domain.ErrReviewNoDevServer("drawing is not available in this deployment")
		}
		res, err := r.Draw.Start(ctx, StartDrawInput{
			RequestID: in.RequestID, ReviewID: view.Stored.ID, Sections: in.Sections, Force: true, Reason: reason, Origin: domain.ArtifactOriginRegenerated,
			CreatedBy: in.By, RunReason: reason, ClientKey: in.ClientRequestID,
		})
		if err != nil {
			return out, err
		}
		out.DrawRun = res.Run
		if res.Skipped != "" {
			out.Skipped = res.Skipped
		}
	default:
		if r.Context == nil {
			return out, domain.ErrReviewNoDevServer("the context run is not available in this deployment")
		}
		res, err := r.Context.Start(ctx, StartContextInput{
			RequestID: in.RequestID, Phase: view.Stored.Phase, Force: true, CreatedBy: in.By, Reason: reason, ClientKey: in.ClientRequestID,
			ForceDraw: in.Scope == domain.RegenAll,
		})
		if err != nil {
			return out, err
		}
		out.ContextRun, out.Skipped = res.Run, res.Skipped
	}
	return out, nil
}

func (r *ReviewRegenerator) audit(ctx context.Context, in RegenerateInput, view ReviewView) {
	reviewObserverOf(r.Service.Observer).ReviewRegenerated(string(in.Reason))
	logReviewAudit(ctx, r.Service.logger(), domain.AuditReviewRegenerate, in.RequestID, view.Stored.ID, view.Stored.InputsDigest)
	if r.Service.Audit == nil {
		return
	}
	sections := make([]string, 0, len(in.Sections))
	for _, s := range in.Sections {
		sections = append(sections, string(s))
	}
	_ = r.Service.Audit.Record(ctx, AuditEvent{
		Action: domain.AuditReviewRegenerate, TargetType: "request", TargetID: in.RequestID, RequestID: in.RequestID, Outcome: "allowed",
		Metadata: map[string]any{"review_id": view.Stored.ID, "scope": string(in.Scope), "sections": sections, "reason": string(in.Reason)},
	})
}

// RestoreResult is the drawing put back in force and what differs from the one it replaced.
type RestoreResult struct {
	Artifact domain.ReviewArtifact
	Diff     *domain.ModelDiff
}

// Restore puts an earlier drawing back in force. It calls no agent and costs nothing.
func (r *ReviewRegenerator) Restore(ctx context.Context, requestID, artifactID, note, by string) (RestoreResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return RestoreResult{}, domain.ErrRequestTenantRequired()
	}
	if len([]rune(note)) > domain.MaxRegenNoteRunes {
		return RestoreResult{}, domain.ErrReviewDecisionInvalid("the note is longer than 200 characters")
	}
	target, err := r.Artifacts.Repo.Get(ctx, requestID, artifactID)
	if err != nil {
		return RestoreResult{}, err
	}
	before, hadActive, err := r.Artifacts.Repo.Active(ctx, requestID, target.Key.Section, target.Key.Variant, target.Key.Name)
	if err != nil {
		return RestoreResult{}, err
	}
	restored, err := r.Artifacts.Repo.Activate(ctx, requestID, artifactID)
	if err != nil {
		return RestoreResult{}, err
	}
	out := RestoreResult{Artifact: restored}
	if hadActive && before.ID != restored.ID {
		if d, derr := CompareArtifactModels(before, restored); derr == nil {
			out.Diff = &d
		}
	}
	logReviewAudit(ctx, r.Service.logger(), domain.AuditReviewRestore, requestID, "", "")
	if r.Service.Audit != nil {
		_ = r.Service.Audit.Record(ctx, AuditEvent{
			Action: domain.AuditReviewRestore, TargetType: "request", TargetID: requestID, RequestID: requestID, Outcome: "allowed",
			Metadata: map[string]any{"artifact_id": artifactID, "section": string(target.Key.Section), "variant": target.Key.Variant, "revision": restored.Revision, "by": by, "has_note": strings.TrimSpace(note) != ""},
		})
	}
	return out, nil
}

// CompareArtifactModels says what differs between the models two drawings were made from. It is computed, not asked of an agent.
func CompareArtifactModels(from, to domain.ReviewArtifact) (domain.ModelDiff, error) {
	var a, b domain.DiagramModel
	if err := json.Unmarshal(from.Model, &a); err != nil {
		return domain.ModelDiff{}, err
	}
	if err := json.Unmarshal(to.Model, &b); err != nil {
		return domain.ModelDiff{}, err
	}
	return domain.DiffModels(a, b), nil
}

// Compare reads two drawings of one Request and diffs their models.
func (r *ReviewRegenerator) Compare(ctx context.Context, requestID, fromID, toID string) (domain.ModelDiff, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ModelDiff{}, domain.ErrRequestTenantRequired()
	}
	from, err := r.Artifacts.Repo.Get(ctx, requestID, fromID)
	if err != nil {
		return domain.ModelDiff{}, err
	}
	to, err := r.Artifacts.Repo.Get(ctx, requestID, toID)
	if err != nil {
		return domain.ModelDiff{}, err
	}
	if from.Key.Section != to.Key.Section || from.Key.Variant != to.Key.Variant || from.Key.Name != to.Key.Name {
		return domain.ModelDiff{}, domain.ErrReviewDecisionInvalid("only two drawings of the same diagram can be compared")
	}
	return CompareArtifactModels(from, to)
}
