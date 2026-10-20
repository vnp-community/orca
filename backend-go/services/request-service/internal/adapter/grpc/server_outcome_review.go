package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
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
