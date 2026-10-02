package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// WithTaskSources enables CreateTaskFromSource/GetTaskSource. Without it both
// RPCs answer Unimplemented, so a deployment that has not run migration 0012
// is unaffected.
func (s *Server) WithTaskSources(create *usecase.CreateTaskFromSource, sources usecase.TaskSourceRepository) *Server {
	s.createTaskFromSource = create
	s.taskSources = sources
	return s
}

func (s *Server) CreateTaskFromSource(ctx context.Context, req *taskv1.CreateTaskFromSourceRequest) (*taskv1.CreateTaskFromSourceResponse, error) {
	if s.createTaskFromSource == nil {
		return s.UnimplementedTaskServiceServer.CreateTaskFromSource(ctx, req)
	}
	c := req.GetCreate()
	res, err := s.createTaskFromSource.Execute(ctx, usecase.CreateTaskFromSourceInput{
		CreateTaskInput: usecase.CreateTaskInput{
			Title:     c.GetTitle(),
			ParentID:  c.GetParentId(),
			ProjectID: c.GetProjectId(),
			CreatorID: c.GetCreatorId(),
		},
		Provider: req.GetProvider(),
		Ref:      req.GetRef(),
		URL:      req.GetUrl(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if !res.Created {
		// An existing task may belong to a teammate and be private: returning
		// it to anyone who knows the issue key would bypass task grants.
		if _, err := s.getTask.Execute(ctx, res.Task.ID); err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
	}
	return &taskv1.CreateTaskFromSourceResponse{Task: toProtoTask(res.Task), Created: res.Created}, nil
}

func (s *Server) GetTaskSource(ctx context.Context, req *taskv1.GetTaskSourceRequest) (*taskv1.GetTaskSourceResponse, error) {
	if s.taskSources == nil {
		return s.UnimplementedTaskServiceServer.GetTaskSource(ctx, req)
	}
	// Reading the task enforces the caller's read permission first.
	if _, err := s.getTask.Execute(ctx, req.GetTaskId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err))
	}
	src, ok, err := s.taskSources.GetSource(ctx, tenantID, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "TASK_SOURCE_LOOKUP_FAILED", "failed to read task source", err))
	}
	if !ok {
		return &taskv1.GetTaskSourceResponse{}, nil
	}
	return &taskv1.GetTaskSourceResponse{Found: true, Provider: string(src.Provider), Ref: src.Ref, Url: src.URL}, nil
}
