package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// planTreeCreator is the use case behind CreatePlanTree (an interface so the handler's error mapping is unit-testable).
type planTreeCreator interface {
	Execute(ctx context.Context, in usecase.CreatePlanTreeInput) (usecase.CreatePlanTreeResult, error)
}

func (s *Server) WithCreatePlanTree(uc planTreeCreator) *Server {
	s.createPlanTree = uc
	return s
}

// CreatePlanTree is called only by request-service (CR-REQ-012). It skips ResolvePermission
// on purpose: api-gateway must not route it (CR-REQ-016), and the caller owns authorization.
func (s *Server) CreatePlanTree(ctx context.Context, req *taskv1.CreatePlanTreeRequest) (*taskv1.CreatePlanTreeResponse, error) {
	if s.createPlanTree == nil {
		return nil, status.Error(codes.Unimplemented, "method CreatePlanTree not implemented")
	}
	res, err := s.createPlanTree.Execute(ctx, toCreatePlanTreeInput(req))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &taskv1.CreatePlanTreeResponse{Plan: toProtoTask(res.Plan), AlreadyExists: res.AlreadyExists}
	for _, p := range res.Phases {
		out.Phases = append(out.Phases, toProtoTask(p))
	}
	for _, t := range res.Tasks {
		out.Tasks = append(out.Tasks, toProtoTask(t))
	}
	return out, nil
}

func toCreatePlanTreeInput(req *taskv1.CreatePlanTreeRequest) usecase.CreatePlanTreeInput {
	in := usecase.CreatePlanTreeInput{
		RequestID: req.GetRequestId(), ProjectID: req.GetProjectId(), Title: req.GetTitle(), Description: req.GetDescription(),
		AIPlanJSON: req.GetAiPlanJson(), CreatorID: req.GetCreatorId(), SupersedesPlanID: req.GetSupersedesPlanId(),
		Tasks: toPlanTreeTasks(req.GetTasks()),
	}
	for _, p := range req.GetPhases() {
		in.Phases = append(in.Phases, usecase.PlanTreePhase{
			Title: p.GetTitle(), Description: p.GetDescription(),
			Tasks: toPlanTreeTasks(p.GetTasks()), DependsOnPhaseIndices: toIntSlice(p.GetDependsOnPhaseIndices()),
		})
	}
	return in
}

func toPlanTreeTasks(in []*taskv1.PlanTreeTask) []usecase.PlanTreeTask {
	var out []usecase.PlanTreeTask
	for _, t := range in {
		pt := usecase.PlanTreeTask{
			Title: t.GetTitle(), Description: t.GetDescription(), Type: t.GetTaskType(),
			PromptTemplate: t.GetPromptTemplate(), AIContext: t.GetAiContext(), Labels: t.GetLabels(),
			DependsOnIndices: toIntSlice(t.GetDependsOnIndices()),
		}
		if t.GetEstimatedHours() != nil {
			v := t.GetEstimatedHours().GetValue()
			pt.EstimatedHours = &v
		}
		out = append(out, pt)
	}
	return out
}
