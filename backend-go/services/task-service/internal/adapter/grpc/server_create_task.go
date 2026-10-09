package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// toCreateTaskInput forwards every client-settable CreateTaskRequest field;
// dropping them silently used to make task_type/priority unsettable over gRPC.
func toCreateTaskInput(ctx context.Context, req *taskv1.CreateTaskRequest) usecase.CreateTaskInput {
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
	// The creator is the authenticated caller. A client-supplied creator_id would let anyone mint an owner grant for
	// somebody else; it is only honoured when there is no caller (system-initiated creation).
	if userID, ok := tenant.UserID(ctx); ok {
		in.CreatorID = userID
	}
	if req.GetEstimatedHours() != nil {
		v := req.GetEstimatedHours().GetValue()
		in.EstimatedHours = &v
	}
	return in
}
