package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// WithTaskSpecs enables SetTaskSpec/GetTaskSpecs/LockTaskSpecs; without it they answer
// Unimplemented, so a deployment that has not run migration 0020 is unaffected.
func (s *Server) WithTaskSpecs(set *usecase.SetTaskSpec, get *usecase.GetTaskSpecs, lock *usecase.LockTaskSpecs) *Server {
	s.setTaskSpec, s.getTaskSpecs, s.lockTaskSpecs = set, get, lock
	return s
}

func toProtoTaskSpec(s domain.TaskSpec) *taskv1.TaskSpec {
	return &taskv1.TaskSpec{
		TaskId: s.TaskID, SchemaVersion: int32(s.SchemaVersion), SpecJson: string(s.Spec),
		Digest: s.Digest, Locked: s.IsLocked(), Version: s.Version,
	}
}

func (s *Server) SetTaskSpec(ctx context.Context, req *taskv1.SetTaskSpecRequest) (*taskv1.SetTaskSpecResponse, error) {
	if s.setTaskSpec == nil {
		return s.UnimplementedTaskServiceServer.SetTaskSpec(ctx, req)
	}
	saved, err := s.setTaskSpec.Execute(ctx, usecase.SetTaskSpecInput{
		TaskID: req.GetTaskId(), SchemaVersion: int(req.GetSchemaVersion()), SpecJSON: []byte(req.GetSpecJson()), ExpectedVersion: req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.SetTaskSpecResponse{Spec: toProtoTaskSpec(saved)}, nil
}

func (s *Server) GetTaskSpecs(ctx context.Context, req *taskv1.GetTaskSpecsRequest) (*taskv1.GetTaskSpecsResponse, error) {
	if s.getTaskSpecs == nil {
		return s.UnimplementedTaskServiceServer.GetTaskSpecs(ctx, req)
	}
	specs, err := s.getTaskSpecs.Execute(ctx, req.GetTaskIds())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.TaskSpec, 0, len(specs))
	for _, sp := range specs {
		out = append(out, toProtoTaskSpec(sp))
	}
	return &taskv1.GetTaskSpecsResponse{Specs: out}, nil
}

func (s *Server) LockTaskSpecs(ctx context.Context, req *taskv1.LockTaskSpecsRequest) (*taskv1.LockTaskSpecsResponse, error) {
	if s.lockTaskSpecs == nil {
		return s.UnimplementedTaskServiceServer.LockTaskSpecs(ctx, req)
	}
	n, err := s.lockTaskSpecs.Execute(ctx, req.GetPlanTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.LockTaskSpecsResponse{Locked: int32(n)}, nil
}
