package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// PlanUseCases are the CR-REQ-012 use cases behind GeneratePlan, GetPlanProposal and CommitPlan.
type PlanUseCases struct {
	Generate *usecase.GeneratePlan
	Get      *usecase.GetPlanProposal
	Commit   *usecase.CommitPlan
	// Bundle reads one file of the authored OpenSpec change; nil when the openspec engine is not configured.
	Bundle *usecase.GetPlanBundleFile
	// Cleanup removes the proposal worktree on request; nil when the openspec engine is not configured.
	Cleanup *usecase.CleanupProposal
	// Retry queues another automatic attempt (CR-REQ-044); nil when automatic Plan generation is off.
	Retry *usecase.RetryPlanGeneration
	// Revisions lists the versions of a Plan (CR-REQ-045).
	Revisions *usecase.ListPlanRevisions
}

// WithPlan attaches the Plan handlers; without it those RPCs stay Unimplemented.
func (s *Server) WithPlan(u PlanUseCases) *Server { s.plan = u; return s }

// GeneratePlan starts the AI run and returns its id; the proposal is read with GetPlanProposal.
func (s *Server) GeneratePlan(ctx context.Context, req *requestv1.GeneratePlanRequest) (*requestv1.GeneratePlanResponse, error) {
	if s.plan.Generate == nil {
		return nil, status.Error(codes.Unimplemented, "GeneratePlan is not wired")
	}
	res, err := s.plan.Generate.Execute(ctx, usecase.GeneratePlanInput{RequestID: req.GetRequestId(), Feedback: req.GetFeedback(), IdempotencyKey: req.GetIdempotencyKey()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GeneratePlanResponse{RunId: res.RunID}, nil
}

func (s *Server) GetPlanProposal(ctx context.Context, req *requestv1.GetPlanProposalRequest) (*requestv1.GetPlanProposalResponse, error) {
	if s.plan.Get == nil {
		return nil, status.Error(codes.Unimplemented, "GetPlanProposal is not wired")
	}
	v, err := s.plan.Get.Execute(ctx, req.GetRequestId(), req.GetRunId(), int(req.GetRevision()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetPlanProposalResponse{
		RunId: v.RunID, Status: v.Status, ErrorCode: v.ErrorCode, ErrorMessage: v.ErrorMessage, RawAiResponse: v.Raw, Engine: v.Engine,
	}
	if v.Proposal != nil {
		out.Proposal = toProtoPlanProposal(*v.Proposal)
	}
	if v.Bundle != nil {
		b := &requestv1.PlanBundle{ChangeId: v.Bundle.ChangeID, Branch: v.Bundle.Branch, Commit: v.Bundle.Commit, TasksMdDigest: v.Bundle.TasksMDDigest,
			Status: v.Bundle.Status, SyncState: v.Bundle.SyncState, SyncError: v.Bundle.SyncError, SyncAttempts: int32(v.Bundle.SyncAttempts)}
		for _, f := range v.Bundle.Files {
			b.Files = append(b.Files, &requestv1.PlanBundleFile{Path: f.Path, Size: f.Size, Omitted: f.Omitted, Sha256: f.SHA256, Change: f.Change})
		}
		out.Bundle = b
	}
	for _, c := range v.Checks {
		out.Checks = append(out.Checks, &requestv1.PlanCheck{Code: c.Code, Where: c.Where, Message: c.Message, Severity: c.Severity})
	}
	if k := v.Knowledge; k != nil {
		out.Knowledge = &requestv1.PlanKnowledge{Available: k.Available, Snapshot: k.Snapshot, Read: k.Read, Gaps: k.Gaps, Conflicts: k.Conflicts, SkillHash: k.SkillHash}
	}
	out.Stats, out.ElapsedMs = toProtoRunStats(v.Stats), v.ElapsedMS
	out.AutoPlan = toProtoPlanAuto(v.AutoPlan)
	return out, nil
}

func (s *Server) GetPlanBundleFile(ctx context.Context, req *requestv1.GetPlanBundleFileRequest) (*requestv1.GetPlanBundleFileResponse, error) {
	if s.plan.Bundle == nil {
		return nil, status.Error(codes.Unimplemented, "GetPlanBundleFile is not wired")
	}
	f, err := s.plan.Bundle.Execute(ctx, req.GetRequestId(), req.GetPath(), int(req.GetRevision()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetPlanBundleFileResponse{Path: f.Path, Content: f.Content, Omitted: f.Omitted}, nil
}

func (s *Server) CommitPlan(ctx context.Context, req *requestv1.CommitPlanRequest) (*requestv1.CommitPlanResponse, error) {
	if s.plan.Commit == nil {
		return nil, status.Error(codes.Unimplemented, "CommitPlan is not wired")
	}
	res, err := s.plan.Commit.Execute(ctx, usecase.CommitPlanInput{RequestID: req.GetRequestId(), Proposal: planProposalFromProto(req.GetProposal())})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CommitPlanResponse{PlanTaskId: res.PlanTaskID, PhaseTaskIds: res.PhaseTaskIDs, TaskIds: res.TaskIDs, AlreadyExists: res.AlreadyExists}, nil
}

func planProposalFromProto(p *requestv1.PlanProposal) domain.PlanProposal {
	if p == nil {
		return domain.PlanProposal{}
	}
	out := domain.PlanProposal{Title: p.GetTitle(), Summary: p.GetSummary(), Notes: p.GetNotes(), Tasks: taskProposalsFromProto(p.GetTasks())}
	for _, ph := range p.GetPhases() {
		out.Phases = append(out.Phases, domain.PhaseProposal{
			Title: ph.GetTitle(), Description: ph.GetDescription(), Tasks: taskProposalsFromProto(ph.GetTasks()),
			DependsOnPhaseIndices: intsFromProto(ph.GetDependsOnPhaseIndices()),
		})
	}
	return out
}

func taskProposalsFromProto(in []*requestv1.TaskProposal) []domain.TaskProposal {
	var out []domain.TaskProposal
	for _, t := range in {
		tp := domain.TaskProposal{
			Title: t.GetTitle(), Description: t.GetDescription(), TaskType: t.GetTaskType(), PromptTemplate: t.GetPromptTemplate(),
			DependsOnIndices: intsFromProto(t.GetDependsOnIndices()), Labels: t.GetLabels(), Irreversible: t.GetIrreversible(),
		}
		if h := t.GetEstimatedHours(); h != nil {
			tp.EstimatedHours = h.GetValue()
		}
		out = append(out, tp)
	}
	return out
}

func intsFromProto(in []int32) []int {
	var out []int
	for _, v := range in {
		out = append(out, int(v))
	}
	return out
}

func int32sToProto(in []int) []int32 {
	var out []int32
	for _, v := range in {
		out = append(out, int32(v))
	}
	return out
}

func toProtoPlanProposal(p domain.PlanProposal) *requestv1.PlanProposal {
	out := &requestv1.PlanProposal{Title: p.Title, Summary: p.Summary, Notes: p.Notes, Tasks: taskProposalsToProto(p.Tasks)}
	for _, ph := range p.Phases {
		out.Phases = append(out.Phases, &requestv1.PhaseProposal{
			Title: ph.Title, Description: ph.Description, Tasks: taskProposalsToProto(ph.Tasks), DependsOnPhaseIndices: int32sToProto(ph.DependsOnPhaseIndices),
		})
	}
	return out
}

func taskProposalsToProto(in []domain.TaskProposal) []*requestv1.TaskProposal {
	var out []*requestv1.TaskProposal
	for _, t := range in {
		tp := &requestv1.TaskProposal{
			Title: t.Title, Description: t.Description, TaskType: t.TaskType, PromptTemplate: t.PromptTemplate,
			DependsOnIndices: int32sToProto(t.DependsOnIndices), Labels: t.Labels, Irreversible: t.Irreversible,
		}
		if t.EstimatedHours > 0 {
			tp.EstimatedHours = wrapperspb.Double(t.EstimatedHours)
		}
		out = append(out, tp)
	}
	return out
}

// CleanupRequestProposal removes the worktree of the Request's proposal and keeps its branch and stored change.
func (s *Server) CleanupRequestProposal(ctx context.Context, req *requestv1.CleanupRequestProposalRequest) (*requestv1.CleanupRequestProposalResponse, error) {
	if s.plan.Cleanup == nil {
		return nil, status.Error(codes.Unimplemented, "CleanupRequestProposal is not wired")
	}
	rel, err := s.plan.Cleanup.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CleanupRequestProposalResponse{Removed: rel.Removed, Abandoned: rel.Abandoned, Branch: rel.Branch}, nil
}
