package grpc

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) ListRequestActivity(ctx context.Context, req *requestv1.ListRequestActivityRequest) (*requestv1.ListRequestActivityResponse, error) {
	if s.execution.Activity == nil {
		return nil, status.Error(codes.Unimplemented, "ListRequestActivity is not wired")
	}
	in := usecase.ListRequestActivityInput{
		RequestID: req.GetRequestId(), PageSize: int(req.GetPageSize()), PageToken: req.GetPageToken(), Kinds: req.GetKinds(),
		MinSeverity: req.GetMinSeverity(), ActorKind: req.GetActorKind(), SubjectID: req.GetSubjectId(),
	}
	if t := req.GetSince(); t != nil {
		at := t.AsTime()
		in.Since = &at
	}
	res, err := s.execution.Activity.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestActivityResponse{NextPageToken: res.NextPageToken}
	for _, e := range res.Entries {
		out.Entries = append(out.Entries, toProtoActivityEntry(e))
	}
	return out, nil
}

func toProtoActivityEntry(e domain.ActivityEntry) *requestv1.ActivityEntry {
	details := "{}"
	if b, err := json.Marshal(e.Details); err == nil && len(b) <= domain.MaxActivityDetailsBytes {
		details = string(b)
	}
	return &requestv1.ActivityEntry{
		Id: e.ID, Kind: e.Kind, OccurredAt: timestamppb.New(e.OccurredAt), ActorKind: e.ActorKind, ActorId: e.ActorID,
		SubjectType: e.SubjectType, SubjectId: e.SubjectID, Summary: e.Summary, Attempt: int32(e.Attempt),
		Severity: string(e.Severity), DetailsJson: details,
	}
}

func (s *Server) GetExecutionSummary(ctx context.Context, req *requestv1.GetExecutionSummaryRequest) (*requestv1.GetExecutionSummaryResponse, error) {
	if s.execution.Summary == nil {
		return nil, status.Error(codes.Unimplemented, "GetExecutionSummary is not wired")
	}
	sum, err := s.execution.Summary.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetExecutionSummaryResponse{Summary: toProtoExecutionSummary(sum)}, nil
}

func toProtoExecutionSummary(sum domain.ExecutionSummary) *requestv1.ExecutionSummary {
	p := sum.Progress
	out := &requestv1.ExecutionSummary{
		RequestId: sum.RequestID, Status: string(sum.Status), Engine: string(sum.Engine), ExecutionMode: string(sum.ExecutionMode),
		Progress: &requestv1.ExecutionProgress{
			PhasesTotal: int32(p.PhasesTotal), PhasesDone: int32(p.PhasesDone), TasksTotal: int32(p.TasksTotal),
			TasksDone: int32(p.TasksDone), TasksFailed: int32(p.TasksFailed), TasksRunning: int32(p.TasksRunning),
		},
		CurrentPhaseId: sum.CurrentPhase, NextPhaseId: sum.NextPhase, MaxAttempts: int32(sum.MaxAttempts),
		Handover: &requestv1.ExecutionHandover{State: string(sum.Handover.State), ArchiveAttempts: int32(sum.Handover.ArchiveAttempts), SummaryRef: sum.Handover.SummaryRef},
	}
	if sum.StartedAt != nil {
		out.StartedAt = timestamppb.New(*sum.StartedAt)
	}
	for _, r := range sum.Running {
		out.Running = append(out.Running, &requestv1.ExecutionRunningTask{
			TaskId: r.TaskID, PhaseId: r.PhaseID, Title: r.Title, Attempt: int32(r.Attempt), MaxAttempts: int32(r.MaxAttempts),
			StartedAt: timestamppb.New(r.StartedAt),
		})
	}
	for _, f := range sum.Failed {
		out.Failed = append(out.Failed, &requestv1.ExecutionFailedTask{
			TaskId: f.TaskID, PhaseId: f.PhaseID, Title: f.Title, Attempts: int32(f.Attempts), MaxAttempts: int32(f.MaxAttempts),
			LastCause: string(f.LastCause), LastError: f.LastError, LastAt: timestamppb.New(f.LastAt),
		})
	}
	for _, n := range sum.NeedsYou {
		item := &requestv1.ExecutionNeedsYou{
			Kind: string(n.Kind), RefId: n.RefID, SubjectType: n.SubjectType, Title: n.Title, PhaseId: n.PhaseID, Since: timestamppb.New(n.Since),
		}
		if n.DueAt != nil {
			item.DueAt = timestamppb.New(*n.DueAt)
		}
		out.NeedsYou = append(out.NeedsYou, item)
	}
	return out
}
