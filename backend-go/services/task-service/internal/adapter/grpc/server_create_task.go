package grpc

import (
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// toCreateTaskInput forwards every client-settable CreateTaskRequest field;
// dropping them silently used to make task_type/priority unsettable over gRPC.
func toCreateTaskInput(req *taskv1.CreateTaskRequest) usecase.CreateTaskInput {
	in := usecase.CreateTaskInput{
		Title:          req.GetTitle(),
		ParentID:       req.GetParentId(),
		ProjectID:      req.GetProjectId(),
		Description:    req.GetDescription(),
		Type:           req.GetTaskType(),
		Priority:       req.GetPriority(),
		AssigneeID:     req.GetAssigneeId(),
		PromptTemplate: req.GetPromptTemplate(),
		AIContext:      req.GetAiContext(),
		Visibility:     req.GetVisibility(),
		RequestID:      req.GetRequestId(),
		Labels:         req.GetLabels(),
		CreatorID:      req.GetCreatorId(),
	}
	if req.GetEstimatedHours() != nil {
		v := req.GetEstimatedHours().GetValue()
		in.EstimatedHours = &v
	}
	return in
}
