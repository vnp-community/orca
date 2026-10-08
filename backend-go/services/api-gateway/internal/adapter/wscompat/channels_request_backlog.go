package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

type backlogArgs struct {
	ProjectID    string   `json:"projectId"`
	RequestID    string   `json:"requestId"`
	PlanTaskID   string   `json:"planTaskId"`
	PhaseTaskID  string   `json:"phaseTaskId"`
	AssigneeID   string   `json:"assigneeId"`
	RequestTypes []string `json:"requestTypes"`
	Categories   []string `json:"categories"`
	PageSize     int32    `json:"pageSize"`
	PageToken    string   `json:"pageToken"`
}

// registerBacklog serves the three views through one RPC; the view is the only
// difference (CR-REQ-015), and each channel forwards only the filters it owns.
func (c requestChannels) registerBacklog(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "backlog.requests", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[backlogArgs](args)
		if err != nil {
			return nil, err
		}
		if err := checkRequestEnum("requestTypes", requestTypeSet, in.RequestTypes...); err != nil {
			return nil, err
		}
		resp, err := c.listBacklog(ctx, &requestv1.ListBacklogRequest{
			View: requestv1.BacklogView_BACKLOG_VIEW_REQUEST, ProjectId: in.ProjectID,
			RequestTypes: in.RequestTypes, Categories: in.Categories,
		}, in)
		if err != nil {
			return nil, err
		}
		return struct {
			RequestRows   []BacklogRequestRowView `json:"requestRows"`
			NextPageToken string                  `json:"nextPageToken"`
		}{backlogRequestRowViews(resp.GetRequestRows()), resp.GetNextPageToken()}, nil
	})
	c.handle(r, "backlog.tasks", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[backlogArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.listBacklog(ctx, &requestv1.ListBacklogRequest{
			View: requestv1.BacklogView_BACKLOG_VIEW_TASK, ProjectId: in.ProjectID, RequestId: in.RequestID,
			PlanTaskId: in.PlanTaskID, AssigneeId: in.AssigneeID,
		}, in)
		if err != nil {
			return nil, err
		}
		return backlogGroupsPage{backlogGroupViews(resp.GetTaskGroups()), resp.GetNextPageToken()}, nil
	})
	c.handle(r, "backlog.execute", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[backlogArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.listBacklog(ctx, &requestv1.ListBacklogRequest{
			View: requestv1.BacklogView_BACKLOG_VIEW_EXECUTE, ProjectId: in.ProjectID, RequestId: in.RequestID,
			PhaseTaskId: in.PhaseTaskID, AssigneeId: in.AssigneeID,
		}, in)
		if err != nil {
			return nil, err
		}
		return backlogGroupsPage{backlogGroupViews(resp.GetExecuteGroups()), resp.GetNextPageToken()}, nil
	})
}

type backlogGroupsPage struct {
	Groups        []BacklogGroupView `json:"groups"`
	NextPageToken string             `json:"nextPageToken"`
}

// listBacklog leaves tenant_id and user_id empty: both come from session metadata (CONTRACT C2).
func (c requestChannels) listBacklog(ctx context.Context, rpc *requestv1.ListBacklogRequest, in backlogArgs) (*requestv1.ListBacklogResponse, error) {
	rpc.PageSize = clampRequestPageSize(in.PageSize)
	rpc.PageToken = in.PageToken
	return c.req.ListBacklog(ctx, rpc)
}
