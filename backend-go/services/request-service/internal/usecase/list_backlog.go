package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListBacklogInput struct {
	View       domain.BacklogView
	ProjectID  string
	Types      []string
	Categories []string
	RequestID  string
	PageToken  string
	PageSize   int
}

type ListBacklogOutput struct {
	RequestRows   []domain.BacklogRequestRow
	TaskGroups    []BacklogGroup
	ExecuteGroups []BacklogGroup
	NextPageToken string
}

type RequestVisibility interface {
	Filter(ctx context.Context, actor domain.DecisionActor, requests []domain.Request) ([]domain.Request, error)
}

type ListBacklog struct {
	Visibility  RequestVisibility
	ListReqsUC  *ListBacklogRequests
	ListTasksUC *ListBacklogTasks
}

func (uc *ListBacklog) Execute(ctx context.Context, in ListBacklogInput) (ListBacklogOutput, error) {
	_, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListBacklogOutput{}, err
	}

	if in.View == domain.BacklogViewUnspecified {
		return ListBacklogOutput{}, apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BACKLOG_INVALID_VIEW", "invalid view", nil)
	}

	uID, _ := tenant.UserID(ctx)
	rRole, _ := tenant.Role(ctx)
	actor := domain.DecisionActor{
		UserID: uID,
		Role:   rRole,
	}

	var out ListBacklogOutput

	if in.View == domain.BacklogViewRequest {
		reqs, next, err := uc.ListReqsUC.Execute(ctx, ListBacklogRequestsInput{
			ProjectID:  in.ProjectID,
			Types:      in.Types,
			Categories: in.Categories,
			PageToken:  in.PageToken,
			PageSize:   in.PageSize,
		})
		if err != nil {
			return ListBacklogOutput{}, err
		}

		// Filter visibility
		var plainReqs []domain.Request
		for _, r := range reqs {
			plainReqs = append(plainReqs, r.Request)
		}
		filtered, err := uc.Visibility.Filter(ctx, actor, plainReqs)
		if err != nil {
			return ListBacklogOutput{}, err
		}

		allowedIDs := make(map[string]bool)
		for _, r := range filtered {
			allowedIDs[r.ID] = true
		}

		var finalReqs []domain.BacklogRequestRow
		for _, r := range reqs {
			if allowedIDs[r.Request.ID] {
				finalReqs = append(finalReqs, r)
			}
		}

		out.RequestRows = finalReqs
		out.NextPageToken = next
	} else {
		// Task or Execute view
		// This uses ListBacklogTasks which theoretically filters requests as well
		// We can just call it and return
		groups, next, err := uc.ListTasksUC.Execute(ctx, ListBacklogTasksInput{
			ProjectID:    in.ProjectID,
			RequestTypes: in.Types,
			RequestID:    in.RequestID,
			PageToken:    in.PageToken,
			PageSize:     in.PageSize,
		})
		if err != nil {
			// Wrap Unavailable if needed (done in adapter/grpc)
			return ListBacklogOutput{}, err
		}

		// Filter visibility
		// We need to fetch the request to filter... for stub we assume groups are filtered in ListTasks or we skip
		if in.View == domain.BacklogViewTask {
			out.TaskGroups = groups
		} else {
			out.ExecuteGroups = groups
		}
		out.NextPageToken = next
	}

	return out, nil
}
