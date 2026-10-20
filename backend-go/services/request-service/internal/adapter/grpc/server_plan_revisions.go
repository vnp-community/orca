package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListPlanRevisions lists the versions of a Request's Plan, newest first (CR-REQ-045).
func (s *Server) ListPlanRevisions(ctx context.Context, req *requestv1.ListPlanRevisionsRequest) (*requestv1.ListPlanRevisionsResponse, error) {
	if s.plan.Revisions == nil {
		return nil, status.Error(codes.Unimplemented, "ListPlanRevisions is not wired")
	}
	page, err := s.plan.Revisions.Execute(ctx, req.GetRequestId(), int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoPlanRevisions(page), nil
}

func toProtoPlanRevisions(page usecase.PlanRevisionsPage) *requestv1.ListPlanRevisionsResponse {
	out := &requestv1.ListPlanRevisionsResponse{NextPageToken: page.NextPageToken}
	for _, r := range page.Revisions {
		out.Revisions = append(out.Revisions, &requestv1.PlanRevision{
			Revision: int32(r.Revision), RunId: r.RunID, Status: r.Status, Engine: r.Engine, RevisionFeedback: r.RevisionFeedback, ActorId: r.ActorID,
			GeneratedAt: timestamppb.New(r.GeneratedAt), Commit: r.Commit, FilesChanged: int32(r.FilesChanged), FileCount: int32(r.FileCount),
			TextKept: r.TextKept, Stats: toProtoRunStats(r.Stats),
		})
	}
	return out
}
