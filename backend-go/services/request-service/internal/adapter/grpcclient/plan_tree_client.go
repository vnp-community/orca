package grpcclient

import (
	"context"
	"fmt"
	"time"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const planTreeTimeout = 10 * time.Second

// PlanTreeClient reads a Plan's subtree from task-service for GetArtifactGraph.
type PlanTreeClient struct {
	client taskv1.TaskServiceClient
}

var _ usecase.PlanTreeReader = (*PlanTreeClient)(nil)

func NewPlanTreeClient(c taskv1.TaskServiceClient) *PlanTreeClient { return &PlanTreeClient{client: c} }

func (c *PlanTreeClient) GetSubtree(ctx context.Context, planTaskID string) (usecase.PlanSubtree, error) {
	ctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return usecase.PlanSubtree{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, planTreeTimeout)
	defer cancel()
	res, err := c.client.GetSubtree(ctx, &taskv1.GetSubtreeRequest{RootId: planTaskID})
	if err != nil {
		return usecase.PlanSubtree{}, fmt.Errorf("grpcclient: get subtree: %w", err)
	}
	var out usecase.PlanSubtree
	for _, t := range res.GetTasks() {
		kind := "task"
		switch t.GetTaskType() {
		case "plan":
			kind = "plan"
		case "phase":
			kind = "phase"
		}
		out.Nodes = append(out.Nodes, usecase.SubtreeNode{ID: t.GetId(), ParentID: t.GetParentId(), Kind: kind, Labels: t.GetLabels()})
	}
	for _, e := range res.GetDependsOnEdges() {
		out.DependsOn = append(out.DependsOn, domain.DependsEdge{From: e.GetFromTaskId(), To: e.GetToTaskId()})
	}
	return out, nil
}
