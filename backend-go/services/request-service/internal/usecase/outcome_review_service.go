package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewAuditor is the slice of AuditRecorder the review needs.
type ReviewAuditor interface {
	Record(ctx context.Context, e AuditEvent) error
}

// ReviewSettingsReader returns the mode a project saved for its outcome review; empty when it saved none.
type ReviewSettingsReader interface {
	OutcomeReviewMode(ctx context.Context, projectID string) (domain.OutcomeReviewMode, error)
}

// OutcomeReviewService builds, stores and reads reviews. It never calls an agent: the context and draw runs are separate use
// cases that update a review afterwards, so a review always opens with the facts Orca measured.
type OutcomeReviewService struct {
	Requests  RequestReader
	Reviews   OutcomeReviewRepository
	Collector *ReviewFactsCollector
	Settings  ReviewSettingsReader
	Config    domain.OutcomeReviewConfig
	// Redact removes secrets from free text before it is stored; nil leaves text as it is.
	Redact func(string) string
	// Audit may be nil; it receives one event for each review that is built or rebuilt.
	Audit ReviewAuditor
	// Artifacts, Profiles and Runs are optional. With them a read tells, for each diagram, whether its drawing is kept, missing, or being
	// drawn now; without them every diagram reads as missing, which is true of a service that draws nothing.
	Artifacts *ReviewArtifactStore
	Profiles  DrawingProfiles
	Runs      OutcomeReviewRunRepository
	// Decisions is optional; with it a regeneration knows which items already have a decision.
	Decisions OutcomeReviewDecisionRepository
	Now       func() time.Time
	Log       *slog.Logger
}

func (s *OutcomeReviewService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *OutcomeReviewService) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// ReviewView is a stored review with its model decoded and the row's identity filled in.
type ReviewView struct {
	Stored StoredReview
	Review domain.OutcomeReview
}

func decodeReview(st StoredReview) (ReviewView, error) {
	var r domain.OutcomeReview
	if err := json.Unmarshal(st.Model, &r); err != nil {
		return ReviewView{}, fmt.Errorf("outcome review %s: model is unreadable: %w", st.ID, err)
	}
	r.ReviewID, r.RequestID, r.Phase, r.State = st.ID, st.RequestID, st.Phase, st.State
	return ReviewView{Stored: st, Review: r}, nil
}

// requireReadyRequest loads the Request and refuses the ones a review cannot be about: the project has the feature off, or the
// Request has no plan yet or was cancelled.
func (s *OutcomeReviewService) requireReadyRequest(ctx context.Context, requestID string) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return domain.Request{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := s.Requests.Get(ctx, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if !s.mode(ctx, req.ProjectID).Enabled() {
		return domain.Request{}, domain.ErrReviewDisabled()
	}
	switch req.Status {
	case domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting, domain.RequestStatusCompleted:
		return req, nil
	}
	return domain.Request{}, domain.ErrReviewNotReady(req.Status)
}

// mode is the kill switch, then the project's saved choice, then the deployment default. An unreadable setting reads as off.
func (s *OutcomeReviewService) mode(ctx context.Context, projectID string) domain.OutcomeReviewMode {
	var saved domain.OutcomeReviewMode
	if s.Settings != nil {
		m, err := s.Settings.OutcomeReviewMode(ctx, projectID)
		if err != nil {
			s.logger().WarnContext(ctx, "outcome review: project setting unreadable; treating the review as off", slog.String("project_id", projectID), slog.Any("error", err))
			return domain.OutcomeReviewOff
		}
		saved = m
	}
	return s.Config.Effective(saved)
}

// Enabled lets callers (the UI flag, the gateway) ask without building anything.
func (s *OutcomeReviewService) Enabled(ctx context.Context, projectID string) bool {
	return s.mode(ctx, projectID).Enabled()
}

// GenerateInput names what to build. An empty Phase lets the service pick from the state of execution.
type GenerateInput struct {
	RequestID string
	Phase     domain.ReviewPhase
	// CreatedBy is the person who asked.
	CreatedBy string
}

type GenerateResult struct {
	View ReviewView
	// Reused is true when a review with the same inputs already existed and nothing was written.
	Reused bool
	// Created is true when a new row was inserted.
	Created bool
	// Collected is what the facts were built from, so a context run does not collect them a second time.
	Collected Collected
}

// Generate builds the review from the facts Orca measures and stores it. The same inputs give the same stored review: asking
// twice writes nothing the second time.
func (s *OutcomeReviewService) Generate(ctx context.Context, in GenerateInput) (GenerateResult, error) {
	req, err := s.requireReadyRequest(ctx, in.RequestID)
	if err != nil {
		return GenerateResult{}, err
	}
	col, err := s.Collector.Collect(ctx, req, in.Phase)
	if err != nil {
		return GenerateResult{}, err
	}
	built, err := domain.BuildOutcomeReview(req.ID, domain.ReviewBuildInput{Facts: col.Facts, KG: domain.KGInfo{Available: false}, Redact: s.Redact})
	if err != nil {
		return GenerateResult{}, err
	}
	headKey := domain.HeadKey(built.Phase, built.Subject.Heads)

	existing, found, err := s.Reviews.GetByKey(ctx, req.ID, built.Phase, headKey)
	if err != nil {
		return GenerateResult{}, err
	}
	if found && existing.InputsDigest == built.InputsDigest && existing.State != domain.ReviewStateStale {
		view, derr := decodeReview(existing)
		return GenerateResult{View: view, Reused: true, Collected: col}, derr
	}

	model, err := encodeReviewModel(&built)
	if err != nil {
		return GenerateResult{}, err
	}

	row := StoredReview{
		RequestID: req.ID, Phase: built.Phase, HeadKey: headKey, HeadCommit: built.Subject.HeadCommit, BaseCommit: built.Subject.BaseCommit,
		InputsDigest: built.InputsDigest, State: domain.ReviewStateReady, Model: model, CreatedBy: in.CreatedBy,
	}
	var stored StoredReview
	created := false
	if found {
		// The facts changed, so the context built on the old ones is gone with them; the locked design is kept for the next run.
		row.ID, row.DesignDigest = existing.ID, existing.DesignDigest
		if stored, err = s.Reviews.Update(ctx, row, existing.Version); err != nil {
			return GenerateResult{}, err
		}
	} else {
		row.ID = uuid.NewString()
		if stored, err = s.Reviews.Create(ctx, row); err != nil {
			if domainCode(err) == "REQUEST_REVIEW_ALREADY_EXISTS" { // lost a race with another Generate: its row is the answer
				if again, ok, gerr := s.Reviews.GetByKey(ctx, req.ID, built.Phase, headKey); gerr == nil && ok {
					view, derr := decodeReview(again)
					return GenerateResult{View: view, Reused: true, Collected: col}, derr
				}
			}
			return GenerateResult{}, err
		}
		created = true
	}
	if built.Phase == domain.ReviewPhaseDelivered {
		if err := s.Reviews.SupersedeOthers(ctx, req.ID, stored.ID); err != nil {
			return GenerateResult{}, err
		}
	}
	s.auditGenerate(ctx, req, stored, created)
	view, err := decodeReview(stored)
	return GenerateResult{View: view, Created: created, Collected: col}, err
}

// encodeReviewModel marshals the review and, when it is over the size limit, cuts the items in half once and says so.
func encodeReviewModel(built *domain.OutcomeReview) ([]byte, error) {
	model, err := json.Marshal(built)
	if err != nil {
		return nil, err
	}
	if len(model) > domain.MaxReviewModelBytes {
		built.Items, built.Truncated = built.Items[:len(built.Items)/2], true
		built.Warnings = append(built.Warnings, "truncated:model")
		if model, err = json.Marshal(built); err != nil {
			return nil, err
		}
		if len(model) > domain.MaxReviewModelBytes {
			return nil, domain.ErrReviewResultInvalid("the review model is larger than the limit even after cutting")
		}
	}
	return model, nil
}

func (s *OutcomeReviewService) auditGenerate(ctx context.Context, req domain.Request, st StoredReview, created bool) {
	if s.Audit == nil {
		return
	}
	// The event names the inputs, never the content: the review may quote text from the Request.
	_ = s.Audit.Record(ctx, AuditEvent{
		Action: domain.AuditReviewGenerate, TargetType: "request", TargetID: req.ID, RequestID: req.ID, Outcome: "allowed",
		Metadata: map[string]any{"review_id": st.ID, "phase": string(st.Phase), "created": created, "inputs_digest": st.InputsDigest},
	})
}

func domainCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// Get reads a review. A review whose database facts changed since it was built is reported stale and the state is saved, so the
// list shows it too. A read never asks git or an agent.
func (s *OutcomeReviewService) Get(ctx context.Context, reviewID string) (ReviewView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ReviewView{}, domain.ErrRequestTenantRequired()
	}
	st, err := s.Reviews.Get(ctx, reviewID)
	if err != nil {
		return ReviewView{}, err
	}
	v, err := s.withStaleness(ctx, st)
	if err != nil {
		return ReviewView{}, err
	}
	return s.annotateDiagrams(ctx, v), nil
}

// Latest returns the newest review of a Request that is not superseded; ok is false when there is none yet.
func (s *OutcomeReviewService) Latest(ctx context.Context, requestID string) (ReviewView, bool, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ReviewView{}, false, domain.ErrRequestTenantRequired()
	}
	st, ok, err := s.Reviews.Latest(ctx, requestID)
	if err != nil || !ok {
		return ReviewView{}, false, err
	}
	v, err := s.withStaleness(ctx, st)
	if err != nil {
		return ReviewView{}, false, err
	}
	return s.annotateDiagrams(ctx, v), true, nil
}

func (s *OutcomeReviewService) withStaleness(ctx context.Context, st StoredReview) (ReviewView, error) {
	view, err := decodeReview(st)
	if err != nil {
		return ReviewView{}, err
	}
	if st.State != domain.ReviewStateReady && st.State != domain.ReviewStateDraft {
		return view, nil
	}
	req, rerr := s.Requests.Get(ctx, st.RequestID)
	if rerr != nil {
		return view, nil // the Request is gone or unreadable; show the review as it was stored
	}
	col, cerr := s.Collector.CollectDatabaseFacts(ctx, req, st.Phase)
	if cerr != nil {
		return view, nil
	}
	now, derr := domain.FactsDigest(st.Phase, col.Facts)
	if derr != nil || view.Review.FactsDigest == "" || now == view.Review.FactsDigest {
		return view, nil
	}
	if marked, serr := s.Reviews.SetState(ctx, st.ID, domain.ReviewStateStale, st.Version); serr == nil {
		st = marked
	} else {
		st.State = domain.ReviewStateStale // a lost race still means the review is stale for this reader
	}
	view.Stored, view.Review.State = st, domain.ReviewStateStale
	return view, nil
}

// List returns a Request's reviews, newest first.
func (s *OutcomeReviewService) List(ctx context.Context, requestID string, pageSize int, afterID string) ([]ReviewView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, domain.ErrRequestTenantRequired()
	}
	rows, err := s.Reviews.List(ctx, requestID, pageSize, afterID)
	if err != nil {
		return nil, err
	}
	out := make([]ReviewView, 0, len(rows))
	for _, r := range rows {
		v, derr := decodeReview(r)
		if derr != nil {
			return nil, derr
		}
		out = append(out, v)
	}
	return out, nil
}

// ReviewEstimate is what a person sees before asking: nothing here costs an agent run.
type ReviewEstimate struct {
	Phase    domain.ReviewPhase
	Finished bool
	Exists   bool
	ReviewID string
	// Stale is true when a review exists and the code or the database facts differ from what it was built on.
	Stale bool
	// CurrentInputsDigest is what a Generate now would build on, so a client can tell whether asking would change anything.
	CurrentInputsDigest string
	Warnings            []string
}

// Estimate answers "what would Generate do" using the database and git-gateway only.
func (s *OutcomeReviewService) Estimate(ctx context.Context, requestID string) (ReviewEstimate, error) {
	req, err := s.requireReadyRequest(ctx, requestID)
	if err != nil {
		return ReviewEstimate{}, err
	}
	col, err := s.Collector.Collect(ctx, req, "")
	if err != nil {
		return ReviewEstimate{}, err
	}
	built, err := domain.BuildOutcomeReview(req.ID, domain.ReviewBuildInput{Facts: col.Facts})
	if err != nil {
		return ReviewEstimate{}, err
	}
	est := ReviewEstimate{Phase: built.Phase, Finished: col.Finished, CurrentInputsDigest: built.InputsDigest, Warnings: built.Warnings}
	existing, found, err := s.Reviews.GetByKey(ctx, req.ID, built.Phase, domain.HeadKey(built.Phase, built.Subject.Heads))
	if err != nil {
		return ReviewEstimate{}, err
	}
	if found {
		est.Exists, est.ReviewID = true, existing.ID
		est.Stale = existing.InputsDigest != built.InputsDigest || existing.State == domain.ReviewStateStale
	}
	return est, nil
}

// annotateDiagrams sets the status of each diagram from what the store holds. It never fails the read: a store that cannot be asked
// leaves the diagrams as they were saved, which says "missing".
func (s *OutcomeReviewService) annotateDiagrams(ctx context.Context, v ReviewView) ReviewView {
	if s.Artifacts == nil || len(v.Review.Diagrams) == 0 {
		return v
	}
	profile := domain.DefaultDrawingProfile()
	if s.Profiles != nil {
		if req, err := s.Requests.Get(ctx, v.Stored.RequestID); err == nil {
			if p, perr := s.Profiles.Resolve(ctx, req.ProjectID); perr == nil {
				profile = p
			}
		}
	}
	refs, err := s.Artifacts.Annotate(ctx, v.Stored.RequestID, v.Review.Diagrams, profile)
	if err != nil {
		s.logger().WarnContext(ctx, "outcome review: drawings could not be looked up", slog.String("review_id", v.Stored.ID), slog.Any("error", err))
		return v
	}
	if s.Runs != nil {
		if run, active, rerr := s.Runs.ActiveForRequest(ctx, v.Stored.RequestID, ReviewRunDraw); rerr == nil && active && run.ReviewID == v.Stored.ID {
			for i := range refs {
				if refs[i].Status == "missing" {
					refs[i] = refs[i].WithStatus("drawing", "")
				}
			}
		}
	}
	v.Review.Diagrams = refs
	if raw, merr := json.Marshal(v.Review); merr == nil {
		v.Stored.Model = raw
	}
	return v
}
