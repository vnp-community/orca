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
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ListBacklog is read-only. The caller comes from the request metadata: tenant_id and user_id in the message are
// ignored so a client cannot ask for somebody else's view.
func (s *Server) ListBacklog(ctx context.Context, req *requestv1.ListBacklogRequest) (*requestv1.ListBacklogResponse, error) {
	if s.execution.Backlog == nil {
		return nil, status.Error(codes.Unimplemented, "ListBacklog is not wired")
	}
	res, err := s.execution.Backlog.Execute(ctx, usecase.ListBacklogInput{
		View: backlogViewFromProto(req.GetView()), ProjectID: req.GetProjectId(), Types: req.GetRequestTypes(), Categories: req.GetCategories(),
		RequestID: req.GetRequestId(), PlanTaskID: req.GetPlanTaskId(), PhaseTaskID: req.GetPhaseTaskId(), AssigneeID: req.GetAssigneeId(),
		PageToken: req.GetPageToken(), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListBacklogResponse{NextPageToken: res.NextPageToken}
	for _, row := range res.RequestRows {
		out.RequestRows = append(out.RequestRows, toProtoBacklogRequestRow(row))
	}
	for _, g := range res.TaskGroups {
		out.TaskGroups = append(out.TaskGroups, toProtoBacklogGroup(g))
	}
	for _, g := range res.ExecuteGroups {
		out.ExecuteGroups = append(out.ExecuteGroups, toProtoBacklogGroup(g))
	}
	return out, nil
}

func backlogViewFromProto(v requestv1.BacklogView) domain.BacklogView {
	switch v {
	case requestv1.BacklogView_BACKLOG_VIEW_REQUEST:
		return domain.BacklogViewRequest
	case requestv1.BacklogView_BACKLOG_VIEW_TASK:
		return domain.BacklogViewTask
	case requestv1.BacklogView_BACKLOG_VIEW_EXECUTE:
		return domain.BacklogViewExecute
	}
	return domain.BacklogViewUnspecified
}

func toProtoBacklogRequestRow(row domain.BacklogRequestRow) *requestv1.BacklogRequestRow {
	r := row.Request
	out := &requestv1.BacklogRequestRow{
		Id: r.ID, ProjectId: r.ProjectID, Type: string(r.Type), ReporterId: r.ReporterID, Stage: string(r.ReturnedFromStage), Status: string(r.Status),
		ReturnedCategory: string(r.ReturnedCategory), UpdatedAt: timestamppb.New(r.UpdatedAt), ReturnedBy: row.ReturnedBy,
		ParentRequestIds: row.ParentRequestIDs, Title: r.Title, Priority: string(r.Urgency), Number: r.Number,
		SourceProvider: string(r.SourceProvider), SourceRef: r.SourceRef, SourceUrl: r.SourceURL, ReturnReason: r.ReturnReason,
	}
	if !row.ReturnedAt.IsZero() {
		out.ReturnedAt = timestamppb.New(row.ReturnedAt)
	}
	return out
}

func toProtoBacklogGroup(g usecase.BacklogGroup) *requestv1.BacklogGroup {
	out := &requestv1.BacklogGroup{
		RequestId: g.RequestID, PlanId: g.PlanID, PhaseId: g.PhaseID, GroupStatus: g.GroupStatus, GroupTitle: g.GroupTitle,
		TotalTasks: int32(g.TotalTasks), PlanTitle: g.PlanTitle, PhaseTitle: g.PhaseTitle, GateStatus: string(g.GateStatus),
	}
	for _, t := range g.Tasks {
		row := &requestv1.BacklogTaskRow{
			Id: t.Task.ID, Type: t.Task.Type, Status: t.Task.Status, Title: t.Task.Title, AssigneeId: t.Task.AssigneeID,
			LastEngine: t.LastEngine, LastLinkStatus: t.LastLinkStatus, FailedAttempts: int32(t.FailedAttempts), LastError: t.LastError,
			BlockedByTaskIds: t.BlockedByTaskIDs,
		}
		if t.Task.EstimatedHours != nil {
			row.EstimatedHours = wrapperspb.Double(*t.Task.EstimatedHours)
		}
		out.Tasks = append(out.Tasks, row)
	}
	return out
}
