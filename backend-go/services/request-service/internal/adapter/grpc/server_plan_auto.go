package grpc

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryPlanGeneration queues another automatic attempt at the Plan after one failed or gave up, or returns the job that
// is already working on it.
func (s *Server) RetryPlanGeneration(ctx context.Context, req *requestv1.RetryPlanGenerationRequest) (*requestv1.RetryPlanGenerationResponse, error) {
	if s.plan.Retry == nil {
		return nil, status.Error(codes.Unimplemented, "RetryPlanGeneration is not wired")
	}
	view, err := s.plan.Retry.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.RetryPlanGenerationResponse{AutoPlan: toProtoPlanAuto(&view)}, nil
}

// toProtoPlanAuto is nil for a server without automatic generation, so clients can tell it from "switched off".
func toProtoPlanAuto(v *usecase.PlanAutoView) *requestv1.PlanAutoState {
	if v == nil {
		return nil
	}
	out := &requestv1.PlanAutoState{
		Enabled: v.Enabled, State: v.State, Origin: v.Origin, Attempt: int32(v.Attempt), MaxAttempts: int32(v.MaxAttempts), ReasonCode: v.ReasonCode,
	}
	if !v.NextAttemptAt.IsZero() {
		out.NextAttemptAt = v.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	return out
}
