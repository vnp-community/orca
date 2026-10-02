package wscompat

import (
	"context"
	"encoding/json"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// taskSourceView is the camelCase shape of a task's external issue link.
type taskSourceView struct {
	Provider string `json:"provider"`
	Ref      string `json:"ref"`
	URL      string `json:"url,omitempty"`
}

// registerTaskSourceChannels exposes the Jira/Linear/GitHub/GitLab → task link:
//   - task.createFromSource: idempotent "start work on this issue"
//   - task.getSource: the issue a task was started from (null if none)
func registerTaskSourceChannels(r *Registry, client taskv1.TaskServiceClient) {
	r.Register("task.createFromSource", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createFromSourceArgs struct {
			Title     string `json:"title"`
			ParentID  string `json:"parentId"`
			ProjectID string `json:"projectId"`
			Provider  string `json:"provider"`
			Ref       string `json:"ref"`
			URL       string `json:"url"`
		}
		in, err := decodeArg[createFromSourceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateTaskFromSource(ctx, &taskv1.CreateTaskFromSourceRequest{
			Create: &taskv1.CreateTaskRequest{
				TenantId: id.TenantID, Title: in.Title, ParentId: in.ParentID, ProjectId: in.ProjectID,
				// Caller identity mints the owner grant, same as the owner-intrinsic
				// short-circuit expects (TASK-TG-003-02).
				CreatorId: id.UserID,
			},
			Provider: in.Provider, Ref: in.Ref, Url: in.URL,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Task    taskView `json:"task"`
			Created bool     `json:"created"`
		}{Task: toTaskView(resp.GetTask()), Created: resp.GetCreated()}, nil
	})

	r.Register("task.getSource", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getSourceArgs struct {
			TaskID string `json:"taskId"`
		}
		in, err := decodeArg[getSourceArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetTaskSource(ctx, &taskv1.GetTaskSourceRequest{TaskId: in.TaskID})
		if err != nil {
			return nil, err
		}
		if !resp.GetFound() {
			return nil, nil
		}
		return taskSourceView{Provider: resp.GetProvider(), Ref: resp.GetRef(), URL: resp.GetUrl()}, nil
	})
}
