package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListBacklogInput struct {
	View        domain.BacklogView
	ProjectID   string
	Types       []string
	Categories  []string
	RequestID   string
	PlanTaskID  string
	PhaseTaskID string
	AssigneeID  string
	PageToken   string
	PageSize    int
}

type ListBacklogOutput struct {
	RequestRows   []domain.BacklogRequestRow
	TaskGroups    []BacklogGroup
	ExecuteGroups []BacklogGroup
	NextPageToken string
}

// ListBacklog is the read-only entry of the three backlog views.
type ListBacklog struct {
	Requests *ListBacklogRequests
	Tasks    *ListBacklogTasks
}

func (uc *ListBacklog) Execute(ctx context.Context, in ListBacklogInput) (ListBacklogOutput, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ListBacklogOutput{}, domain.ErrRequestTenantRequired()
	}
	switch in.View {
	case domain.BacklogViewRequest:
		rows, next, err := uc.Requests.Execute(ctx, ListBacklogRequestsInput{
			ProjectID: in.ProjectID, Types: in.Types, Categories: in.Categories, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		return ListBacklogOutput{RequestRows: rows, NextPageToken: next}, err
	case domain.BacklogViewTask, domain.BacklogViewExecute:
		groups, next, err := uc.Tasks.Execute(ctx, ListBacklogTasksInput{
			View: in.View, ProjectID: in.ProjectID, RequestTypes: in.Types, RequestID: in.RequestID, PlanTaskID: in.PlanTaskID,
			PhaseTaskID: in.PhaseTaskID, AssigneeID: in.AssigneeID, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return ListBacklogOutput{}, err
		}
		out := ListBacklogOutput{NextPageToken: next}
		if in.View == domain.BacklogViewTask {
			out.TaskGroups = groups
		} else {
			out.ExecuteGroups = groups
		}
		return out, nil
	}
	return ListBacklogOutput{}, domain.ErrBacklogInvalidView()
}
