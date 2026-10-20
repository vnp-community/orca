package grpc

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OutcomeReviewUseCases are the CR-OR-001 use cases behind the review RPCs.
type OutcomeReviewUseCases struct {
	Service *usecase.OutcomeReviewService
	// Context may be nil; the review is then built from the facts alone and a request to run the context is refused.
	Context *usecase.ReviewContextRunner
	// Artifacts may be nil; the drawings are then not served.
	Artifacts *usecase.ReviewArtifactStore
	// Decider, Regenerator and Profiles may be nil; the matching RPCs then answer Unimplemented.
	Decider     *usecase.ReviewDecider
	Regenerator *usecase.ReviewRegenerator
	Profiles    *usecase.ReviewDrawingProfiles
}

// WithOutcomeReview attaches the review handlers; without it those RPCs stay Unimplemented.
func (s *Server) WithOutcomeReview(u OutcomeReviewUseCases) *Server { s.outcomeReview = u; return s }

func (s *Server) outcomeReviewWired() error {
	if s.outcomeReview.Service == nil {
		return status.Error(codes.Unimplemented, "the outcome review is not wired")
	}
	return nil
}

func (s *Server) EstimateOutcomeReview(ctx context.Context, req *requestv1.EstimateOutcomeReviewRequest) (*requestv1.EstimateOutcomeReviewResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	est, err := s.outcomeReview.Service.Estimate(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.EstimateOutcomeReviewResponse{
		Phase: string(est.Phase), ExecutionFinished: est.Finished, ReviewExists: est.Exists, ReviewId: est.ReviewID, Stale: est.Stale,
		CurrentInputsDigest: est.CurrentInputsDigest, Warnings: est.Warnings,
	}
	if run := s.outcomeReview.Context; run != nil {
		ce, cerr := run.EstimateContext(ctx, req.GetRequestId(), est)
		if cerr != nil {
			return nil, apperrors.ToGRPCStatus(cerr)
		}
		out.ContextCached, out.DesignLocked, out.ContextRunnable, out.ContextGaps = ce.Cached, ce.DesignLocked, ce.Runnable, ce.Gaps
	}
	if req.GetScope() != "" && s.outcomeReview.Regenerator != nil {
		sections := make([]domain.ReviewSection, 0, len(req.GetSections()))
		for _, sec := range req.GetSections() {
			sections = append(sections, domain.ReviewSection(sec))
		}
		re, rerr := s.outcomeReview.Regenerator.Estimate(ctx, req.GetRequestId(), domain.RegenRequest{Scope: domain.RegenScope(req.GetScope()), Sections: sections, Reason: domain.RegenOther})
		if rerr != nil {
			return nil, apperrors.ToGRPCStatus(rerr)
		}
		out.Regeneration = toProtoRegenEstimate(re)
	}
	return out, nil
}

func (s *Server) GenerateOutcomeReview(ctx context.Context, req *requestv1.GenerateOutcomeReviewRequest) (*requestv1.GenerateOutcomeReviewResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	phase := domain.ReviewPhase(req.GetPhase())
	if phase != "" && !phase.Valid() {
		return nil, status.Error(codes.InvalidArgument, "phase must be design or delivered")
	}
	if req.GetRunContext() {
		if s.outcomeReview.Context == nil {
			return nil, status.Error(codes.Unimplemented, "the outcome review context run is not wired")
		}
		res, err := s.outcomeReview.Context.Start(ctx, usecase.StartContextInput{
			RequestID: req.GetRequestId(), Phase: phase, Force: req.GetForce(), AllowCodeOnly: req.GetAllowCodeOnly(), CreatedBy: actor,
		})
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		out := &requestv1.GenerateOutcomeReviewResponse{
			Review: toProtoOutcomeReview(res.View.Stored), Reused: res.Reused, Created: res.Created, ContextCached: res.ContextCached, ContextSkipped: res.Skipped,
		}
		if res.Run != nil {
			out.Run = toProtoOutcomeReviewRun(*res.Run)
		}
		out.DrawError = res.DrawError
		if d := res.Draw; d != nil {
			out.DrawingsCached, out.DrawingsPending = int32(d.Cached), int32(d.Pending)
			if d.Run != nil {
				out.DrawRun = toProtoOutcomeReviewRun(*d.Run)
			}
		}
		return out, nil
	}
	res, err := s.outcomeReview.Service.Generate(ctx, usecase.GenerateInput{RequestID: req.GetRequestId(), Phase: phase, CreatedBy: actor})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GenerateOutcomeReviewResponse{Review: toProtoOutcomeReview(res.View.Stored), Reused: res.Reused, Created: res.Created}, nil
}

func (s *Server) GetOutcomeReview(ctx context.Context, req *requestv1.GetOutcomeReviewRequest) (*requestv1.GetOutcomeReviewResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	if id := req.GetReviewId(); id != "" {
		v, err := s.outcomeReview.Service.Get(ctx, id)
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		if v.Stored.RequestID != req.GetRequestId() { // authorized on the Request named; another Request's review is not found
			return nil, apperrors.ToGRPCStatus(domain.ErrReviewNotFound(id))
		}
		return &requestv1.GetOutcomeReviewResponse{Review: toProtoOutcomeReview(v.Stored), Found: true}, nil
	}
	v, ok, err := s.outcomeReview.Service.Latest(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if !ok {
		return &requestv1.GetOutcomeReviewResponse{}, nil
	}
	return &requestv1.GetOutcomeReviewResponse{Review: toProtoOutcomeReview(v.Stored), Found: true}, nil
}

func (s *Server) ListOutcomeReviews(ctx context.Context, req *requestv1.ListOutcomeReviewsRequest) (*requestv1.ListOutcomeReviewsResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	size := int(req.GetPageSize())
	if size <= 0 || size > 100 {
		size = 20
	}
	views, err := s.outcomeReview.Service.List(ctx, req.GetRequestId(), size+1, req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListOutcomeReviewsResponse{}
	for i, v := range views {
		if i == size {
			out.NextPageToken = views[size-1].Stored.ID
			break
		}
		out.Reviews = append(out.Reviews, toProtoOutcomeReview(v.Stored))
	}
	return out, nil
}

func toProtoOutcomeReview(st usecase.StoredReview) *requestv1.OutcomeReview {
	return &requestv1.OutcomeReview{
		ReviewId: st.ID, RequestId: st.RequestID, Phase: string(st.Phase), State: string(st.State), HeadKey: st.HeadKey,
		ModelJson: string(st.Model), KgSnapshot: st.KGSnapshot, Version: st.Version,
		CreatedAt: timestamppb.New(st.CreatedAt), UpdatedAt: timestamppb.New(st.UpdatedAt),
	}
}

func (s *Server) ListOutcomeReviewRuns(ctx context.Context, req *requestv1.ListOutcomeReviewRunsRequest) (*requestv1.ListOutcomeReviewRunsResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Context == nil {
		return &requestv1.ListOutcomeReviewRunsResponse{}, nil
	}
	runs, err := s.outcomeReview.Context.ListRuns(ctx, req.GetRequestId(), req.GetReviewId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListOutcomeReviewRunsResponse{}
	for _, r := range runs {
		out.Runs = append(out.Runs, toProtoOutcomeReviewRun(r))
	}
	return out, nil
}

func toProtoOutcomeReviewRun(r usecase.ReviewRun) *requestv1.OutcomeReviewRun {
	out := &requestv1.OutcomeReviewRun{
		RunId: r.ID, ReviewId: r.ReviewID, RequestId: r.RequestID, Kind: string(r.Kind), Status: string(r.Status), Scope: r.Scope,
		ErrorCode: r.ErrorCode, CostEstimate: r.CostEstimate, CostActual: r.CostActual, StartedAt: timestamppb.New(r.StartedAt),
	}
	if r.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return out
}

func (s *Server) GetReviewArtifact(ctx context.Context, req *requestv1.GetReviewArtifactRequest) (*requestv1.GetReviewArtifactResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Artifacts == nil {
		return nil, status.Error(codes.Unimplemented, "the review drawings are not wired")
	}
	if req.GetRequestId() == "" || req.GetArtifactId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id and artifact_id are required")
	}
	got, err := s.outcomeReview.Artifacts.Read(ctx, req.GetRequestId(), req.GetArtifactId(), req.GetIfDigest())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetReviewArtifactResponse{Artifact: toProtoArtifactMeta(got.Artifact), NotModified: got.NotModified}
	if !got.NotModified && got.Blob.Digest != "" {
		out.MediaType, out.Content = got.Blob.MediaType, string(got.Blob.Content)
	}
	return out, nil
}

func (s *Server) ListReviewArtifactRevisions(ctx context.Context, req *requestv1.ListReviewArtifactRevisionsRequest) (*requestv1.ListReviewArtifactRevisionsResponse, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Artifacts == nil {
		return &requestv1.ListReviewArtifactRevisionsResponse{}, nil
	}
	section := domain.ReviewSection(req.GetSection())
	if req.GetRequestId() == "" || !section.Drawn() || (req.GetVariant() != domain.DiagramVariantDesign && req.GetVariant() != domain.DiagramVariantDelta) {
		return nil, status.Error(codes.InvalidArgument, "request_id, a drawn section and a variant (design or delta) are required")
	}
	rows, err := s.outcomeReview.Artifacts.Revisions(ctx, req.GetRequestId(), section, req.GetVariant(), req.GetName())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListReviewArtifactRevisionsResponse{}
	for _, a := range rows {
		out.Revisions = append(out.Revisions, toProtoArtifactMeta(a))
	}
	return out, nil
}

func toProtoArtifactMeta(a domain.ReviewArtifact) *requestv1.ReviewArtifactMeta {
	return &requestv1.ReviewArtifactMeta{
		ArtifactId: a.ID, RequestId: a.RequestID, Section: string(a.Key.Section), Variant: a.Key.Variant, Name: a.Key.Name, DiagramType: string(a.Type),
		InputDigest: a.Key.InputDigest, PresetDigest: a.Key.PresetDigest, SkillVersion: a.Key.SkillVersion, Revision: int32(a.Revision), State: string(a.State),
		Origin: string(a.Origin), Reason: a.Reason, ErrorCode: a.ErrorCode, BlobDigest: a.BlobDigest, SizeBytes: int32(a.SizeBytes), ViewBox: a.ViewBox,
		Title: a.Title, Desc: a.Desc, CreatedAt: timestamppb.New(a.CreatedAt),
	}
}

func toProtoRegenEstimate(e usecase.RegenEstimate) *requestv1.RegenerationEstimate {
	return &requestv1.RegenerationEstimate{
		AgentRuns: int32(e.AgentRuns), DiagramsToDraw: int32(e.DiagramsToDraw), CostKnown: e.CostKnown, CostUsd: e.CostUSD,
		LastContextCost: e.LastContextCost, LastDrawCost: e.LastDrawCost, ConfirmRequired: e.ConfirmRequired, DrawingsDependOnResult: e.DrawingsDependOnResult,
	}
}

func (s *Server) deciderOrUnimplemented() (*usecase.ReviewDecider, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Decider == nil {
		return nil, status.Error(codes.Unimplemented, "review decisions are not wired")
	}
	return s.outcomeReview.Decider, nil
}

func (s *Server) DecideOutcomeReviewItem(ctx context.Context, req *requestv1.DecideOutcomeReviewItemRequest) (*requestv1.DecideOutcomeReviewItemResponse, error) {
	d, err := s.deciderOrUnimplemented()
	if err != nil {
		return nil, err
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := d.Decide(ctx, usecase.DecideInput{
		RequestID: req.GetRequestId(), ReviewID: req.GetReviewId(), ItemKey: req.GetItemKey(), Decision: domain.ReviewDecision(req.GetDecision()), Note: req.GetNote(),
		ExpectedVersion: req.GetExpectedVersion(), DecidedBy: actor,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.DecideOutcomeReviewItemResponse{
		Decision: toProtoReviewDecision(res.Decision), ReviewVersion: res.Review.Version, ReviewState: string(res.Review.State), OpenDecisions: int32(res.Open),
	}, nil
}

func (s *Server) ApproveOutcomeReview(ctx context.Context, req *requestv1.ApproveOutcomeReviewRequest) (*requestv1.ApproveOutcomeReviewResponse, error) {
	d, err := s.deciderOrUnimplemented()
	if err != nil {
		return nil, err
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	st, err := d.Approve(ctx, usecase.ApproveInput{RequestID: req.GetRequestId(), ReviewID: req.GetReviewId(), Note: req.GetNote(), ExpectedVersion: req.GetExpectedVersion(), ApprovedBy: actor})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ApproveOutcomeReviewResponse{Review: toProtoOutcomeReview(st)}, nil
}

func (s *Server) ListOutcomeReviewDecisions(ctx context.Context, req *requestv1.ListOutcomeReviewDecisionsRequest) (*requestv1.ListOutcomeReviewDecisionsResponse, error) {
	d, err := s.deciderOrUnimplemented()
	if err != nil {
		return nil, err
	}
	out := &requestv1.ListOutcomeReviewDecisionsResponse{}
	if req.GetItemKey() != "" {
		hist, err := d.History(ctx, req.GetRequestId(), req.GetReviewId(), req.GetItemKey())
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		for _, h := range hist {
			out.Decisions = append(out.Decisions, toProtoReviewDecision(h))
		}
		return out, nil
	}
	list, err := d.List(ctx, req.GetRequestId(), req.GetReviewId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	for _, h := range list.InForce {
		out.Decisions = append(out.Decisions, toProtoReviewDecision(h))
	}
	out.OpenDecisions = int32(list.Open)
	return out, nil
}

func (s *Server) LinkOutcomeReviewChild(ctx context.Context, req *requestv1.LinkOutcomeReviewChildRequest) (*requestv1.LinkOutcomeReviewChildResponse, error) {
	d, err := s.deciderOrUnimplemented()
	if err != nil {
		return nil, err
	}
	n, err := d.LinkChild(ctx, req.GetRequestId(), req.GetReviewId(), req.GetItemKeys(), req.GetChildRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.LinkOutcomeReviewChildResponse{Linked: int32(n)}, nil
}

func toProtoReviewDecision(d domain.ItemDecision) *requestv1.OutcomeReviewDecision {
	return &requestv1.OutcomeReviewDecision{
		DecisionId: d.ID, ReviewId: d.ReviewID, ItemKey: d.ItemKey, Decision: string(d.Decision), Note: d.Note, ChildRequestId: d.ChildRequestID,
		DecidedBy: d.DecidedBy, DecidedAt: timestamppb.New(d.DecidedAt), Active: d.Active, Orphaned: d.Orphaned,
	}
}

func (s *Server) regeneratorOrUnimplemented() (*usecase.ReviewRegenerator, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Regenerator == nil {
		return nil, status.Error(codes.Unimplemented, "regeneration is not wired")
	}
	return s.outcomeReview.Regenerator, nil
}

func (s *Server) RegenerateReviewArtifacts(ctx context.Context, req *requestv1.RegenerateReviewArtifactsRequest) (*requestv1.RegenerateReviewArtifactsResponse, error) {
	rg, err := s.regeneratorOrUnimplemented()
	if err != nil {
		return nil, err
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	sections := make([]domain.ReviewSection, 0, len(req.GetSections()))
	for _, sec := range req.GetSections() {
		sections = append(sections, domain.ReviewSection(sec))
	}
	res, err := rg.Regenerate(ctx, usecase.RegenerateInput{
		RequestID:    req.GetRequestId(),
		RegenRequest: domain.RegenRequest{Scope: domain.RegenScope(req.GetScope()), Sections: sections, Reason: domain.RegenReason(req.GetReason()), Note: req.GetNote()},
		ConfirmCost:  req.GetConfirmCost(), ClientRequestID: req.GetClientRequestId(), ExpectedVersion: req.GetExpectedVersion(), By: actor,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.RegenerateReviewArtifactsResponse{Replayed: res.Replayed, Estimate: toProtoRegenEstimate(res.Estimate), Skipped: res.Skipped}
	if res.ContextRun != nil {
		out.ContextRun = toProtoOutcomeReviewRun(*res.ContextRun)
	}
	if res.DrawRun != nil {
		out.DrawRun = toProtoOutcomeReviewRun(*res.DrawRun)
	}
	return out, nil
}

func (s *Server) RestoreReviewArtifactRevision(ctx context.Context, req *requestv1.RestoreReviewArtifactRevisionRequest) (*requestv1.RestoreReviewArtifactRevisionResponse, error) {
	rg, err := s.regeneratorOrUnimplemented()
	if err != nil {
		return nil, err
	}
	actor, _ := tenant.UserID(ctx)
	res, err := rg.Restore(ctx, req.GetRequestId(), req.GetArtifactId(), req.GetNote(), actor)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.RestoreReviewArtifactRevisionResponse{Artifact: toProtoArtifactMeta(res.Artifact)}
	if res.Diff != nil {
		out.Diff, out.HasDiff = toProtoModelDiff(*res.Diff), true
	}
	return out, nil
}

func (s *Server) CompareReviewArtifactRevisions(ctx context.Context, req *requestv1.CompareReviewArtifactRevisionsRequest) (*requestv1.CompareReviewArtifactRevisionsResponse, error) {
	rg, err := s.regeneratorOrUnimplemented()
	if err != nil {
		return nil, err
	}
	diff, err := rg.Compare(ctx, req.GetRequestId(), req.GetFromArtifactId(), req.GetToArtifactId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CompareReviewArtifactRevisionsResponse{Diff: toProtoModelDiff(diff)}, nil
}

func toProtoModelDiff(d domain.ModelDiff) *requestv1.ModelDiff {
	return &requestv1.ModelDiff{AddedNodes: d.AddedNodes, RemovedNodes: d.RemovedNodes, ChangedNodes: d.ChangedNodes, AddedEdges: d.AddedEdges, RemovedEdges: d.RemovedEdges, ChangedEdges: d.ChangedEdges}
}

func (s *Server) profilesOrUnimplemented() (*usecase.ReviewDrawingProfiles, error) {
	if err := s.outcomeReviewWired(); err != nil {
		return nil, err
	}
	if s.outcomeReview.Profiles == nil {
		return nil, status.Error(codes.Unimplemented, "drawing profiles are not wired")
	}
	return s.outcomeReview.Profiles, nil
}

func (s *Server) GetReviewDrawingProfile(ctx context.Context, req *requestv1.GetReviewDrawingProfileRequest) (*requestv1.GetReviewDrawingProfileResponse, error) {
	p, err := s.profilesOrUnimplemented()
	if err != nil {
		return nil, err
	}
	view, err := p.Get(ctx, req.GetProjectId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	effective, err := json.Marshal(view.Effective)
	if err != nil {
		return nil, status.Error(codes.Internal, "the profile in force could not be written")
	}
	out := &requestv1.GetReviewDrawingProfileResponse{EffectiveJson: string(effective), Sources: view.Effective.Sources}
	if view.Tenant != nil {
		out.TenantLayer, out.HasTenantLayer = toProtoProfileLayer(*view.Tenant), true
	}
	if view.Project != nil {
		out.ProjectLayer, out.HasProjectLayer = toProtoProfileLayer(*view.Project), true
	}
	return out, nil
}

func (s *Server) SaveReviewDrawingProfile(ctx context.Context, req *requestv1.SaveReviewDrawingProfileRequest) (*requestv1.SaveReviewDrawingProfileResponse, error) {
	p, err := s.profilesOrUnimplemented()
	if err != nil {
		return nil, err
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	saved, err := p.Save(ctx, req.GetProjectId(), []byte(req.GetSpecJson()), req.GetExpectedVersion(), actor)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.SaveReviewDrawingProfileResponse{Layer: toProtoProfileLayer(saved)}, nil
}

func (s *Server) ResetReviewDrawingProfile(ctx context.Context, req *requestv1.ResetReviewDrawingProfileRequest) (*requestv1.ResetReviewDrawingProfileResponse, error) {
	p, err := s.profilesOrUnimplemented()
	if err != nil {
		return nil, err
	}
	if err := p.Reset(ctx, req.GetProjectId(), req.GetExpectedVersion()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ResetReviewDrawingProfileResponse{}, nil
}

func toProtoProfileLayer(p usecase.StoredDrawingProfile) *requestv1.ReviewDrawingProfileLayer {
	return &requestv1.ReviewDrawingProfileLayer{
		ProjectId: p.ProjectKey, SpecJson: string(p.Spec), SpecDigest: p.SpecDigest, Version: p.Version, UpdatedBy: p.UpdatedBy, UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}
