package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) WithListExecutionStates(uc *usecase.ListExecutionStates) *Server {
	s.listExecutionStates = uc
	return s
}

// ListExecutionStates returns execution status for a list of task IDs.
// It checks the tenant boundary but relies on the caller (e.g. request-service)
// to perform access control, as it may expose states of tasks the caller cannot see.
func (s *Server) ListExecutionStates(ctx context.Context, req *taskv1.ListExecutionStatesRequest) (*taskv1.ListExecutionStatesResponse, error) {
	if s.listExecutionStates == nil {
		return nil, status.Error(codes.Unimplemented, "method ListExecutionStates not implemented")
	}

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err))
	}
	in := usecase.ListExecutionStatesInput{
		TaskIDs: req.TaskIds,
	}

	states, err := s.listExecutionStates.Execute(ctx, tenantID, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}

	res := &taskv1.ListExecutionStatesResponse{
		States: make([]*taskv1.ExecutionState, len(states)),
	}

	for i, st := range states {
		protoState := &taskv1.ExecutionState{
			TaskId:           st.TaskID,
			BlockedByTaskIds: st.BlockedByTaskIDs,
			LastEngine:       st.LastEngine,
			LastLinkStatus:   st.LastLinkStatus,
			FailedAttempts:   int32(st.FailedAttempts),
		}
		if !st.LastStartedAt.IsZero() {
			protoState.LastStartedAt = timestamppb.New(st.LastStartedAt)
		}
		if st.LastCompletedAt != nil {
			protoState.LastCompletedAt = timestamppb.New(*st.LastCompletedAt)
		}
		res.States[i] = protoState
	}

	return res, nil
}
