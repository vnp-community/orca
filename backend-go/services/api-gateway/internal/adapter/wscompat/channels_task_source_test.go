package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"google.golang.org/grpc"
)

type fakeTaskSourceClient struct {
	taskv1.TaskServiceClient
	gotCreate *taskv1.CreateTaskFromSourceRequest
	found     bool
}

func (f *fakeTaskSourceClient) CreateTaskFromSource(_ context.Context, in *taskv1.CreateTaskFromSourceRequest, _ ...grpc.CallOption) (*taskv1.CreateTaskFromSourceResponse, error) {
	f.gotCreate = in
	return &taskv1.CreateTaskFromSourceResponse{Task: &taskv1.Task{Id: "t1", Title: in.GetCreate().GetTitle()}, Created: true}, nil
}

func (f *fakeTaskSourceClient) GetTaskSource(_ context.Context, _ *taskv1.GetTaskSourceRequest, _ ...grpc.CallOption) (*taskv1.GetTaskSourceResponse, error) {
	if !f.found {
		return &taskv1.GetTaskSourceResponse{}, nil
	}
	return &taskv1.GetTaskSourceResponse{Found: true, Provider: "jira", Ref: "ENG-1", Url: "https://x/browse/ENG-1"}, nil
}

func TestTaskCreateFromSourceChannel_ForwardsFieldsAndReturnsCamelCase(t *testing.T) {
	fake := &fakeTaskSourceClient{}
	r := NewRegistry()
	registerTaskSourceChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "task.createFromSource", argsJSON(t, map[string]any{
		"title": "ENG-1 fix", "projectId": "proj-1", "provider": "jira", "ref": "ENG-1", "url": "https://x/browse/ENG-1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotCreate.GetProvider() != "jira" || fake.gotCreate.GetRef() != "ENG-1" || fake.gotCreate.GetCreate().GetProjectId() != "proj-1" {
		t.Errorf("fields not forwarded: %+v", fake.gotCreate)
	}
	if fake.gotCreate.GetCreate().GetCreatorId() != "u1" {
		t.Errorf("want creatorId from identity, got %q", fake.gotCreate.GetCreate().GetCreatorId())
	}
	raw, _ := json.Marshal(result)
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["created"] != true {
		t.Errorf("want created=true on wire, got %s", raw)
	}
	if task, _ := wire["task"].(map[string]any); task["id"] != "t1" {
		t.Errorf("want task.id=t1 on wire, got %s", raw)
	}
}

func TestTaskGetSourceChannel(t *testing.T) {
	r := NewRegistry()
	fake := &fakeTaskSourceClient{}
	registerTaskSourceChannels(r, fake)

	got, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.getSource", argsJSON(t, map[string]any{"taskId": "t1"}))
	if err != nil || got != nil {
		t.Fatalf("want nil result for task without source, got %v, %v", got, err)
	}

	fake.found = true
	got, err = r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.getSource", argsJSON(t, map[string]any{"taskId": "t1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := got.(taskSourceView)
	if !ok || view.Provider != "jira" || view.Ref != "ENG-1" {
		t.Fatalf("unexpected view: %#v", got)
	}
}
