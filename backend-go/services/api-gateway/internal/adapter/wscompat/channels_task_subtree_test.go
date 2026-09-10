// Tests for TASK-TG-01-08's 4 genuinely new task.* channels
// (getSubtree/recalculateProgress/addComment/listComments), named with a
// TestTaskChannels prefix per that task's own Verify section. The
// BUG-034 gap-closure channels (task.list/update/delete/getDependencies)
// are already covered by channels_automation_task_test.go's
// TestTask*Channel_* tests against registerTaskCRUDChannels — see
// channels.go's registerTaskChannels doc comment for why they aren't
// re-registered (and so aren't re-tested) here.
package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// fakeTaskSubtreeClient implements taskv1.TaskServiceClient with canned
// responses for exactly the 4 new RPCs this file tests — embeds the (nil)
// interface so it satisfies every other method, which this file's tests
// never call.
type fakeTaskSubtreeClient struct {
	taskv1.TaskServiceClient

	getSubtreeFunc          func(ctx context.Context, in *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error)
	recalculateProgressFunc func(ctx context.Context, in *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error)
	addCommentFunc          func(ctx context.Context, in *taskv1.AddCommentRequest) (*taskv1.AddCommentResponse, error)
	listCommentsFunc        func(ctx context.Context, in *taskv1.ListCommentsRequest) (*taskv1.ListCommentsResponse, error)
}

func (f *fakeTaskSubtreeClient) GetSubtree(ctx context.Context, in *taskv1.GetSubtreeRequest, _ ...grpc.CallOption) (*taskv1.GetSubtreeResponse, error) {
	return f.getSubtreeFunc(ctx, in)
}

func (f *fakeTaskSubtreeClient) RecalculateProgress(ctx context.Context, in *taskv1.RecalculateProgressRequest, _ ...grpc.CallOption) (*taskv1.RecalculateProgressResponse, error) {
	return f.recalculateProgressFunc(ctx, in)
}

func (f *fakeTaskSubtreeClient) AddComment(ctx context.Context, in *taskv1.AddCommentRequest, _ ...grpc.CallOption) (*taskv1.AddCommentResponse, error) {
	return f.addCommentFunc(ctx, in)
}

func (f *fakeTaskSubtreeClient) ListComments(ctx context.Context, in *taskv1.ListCommentsRequest, _ ...grpc.CallOption) (*taskv1.ListCommentsResponse, error) {
	return f.listCommentsFunc(ctx, in)
}

func TestTaskChannels_GetSubtree(t *testing.T) {
	var gotReq *taskv1.GetSubtreeRequest
	fake := &fakeTaskSubtreeClient{
		getSubtreeFunc: func(ctx context.Context, in *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
			gotReq = in
			return &taskv1.GetSubtreeResponse{Tasks: []*taskv1.Task{{Id: "root"}, {Id: "child"}}}, nil
		},
	}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.getSubtree", argsJSON(t, map[string]any{"rootId": "root"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRootId() != "root" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*taskv1.GetSubtreeResponse)
	if !ok || len(resp.GetTasks()) != 2 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskChannels_RecalculateProgress(t *testing.T) {
	var gotReq *taskv1.RecalculateProgressRequest
	fake := &fakeTaskSubtreeClient{
		recalculateProgressFunc: func(ctx context.Context, in *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error) {
			gotReq = in
			return &taskv1.RecalculateProgressResponse{ProgressPercent: 42}, nil
		},
	}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.recalculateProgress", argsJSON(t, map[string]any{"rootId": "root"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRootId() != "root" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*taskv1.RecalculateProgressResponse)
	if !ok || resp.GetProgressPercent() != 42 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskChannels_AddComment(t *testing.T) {
	var gotReq *taskv1.AddCommentRequest
	fake := &fakeTaskSubtreeClient{
		addCommentFunc: func(ctx context.Context, in *taskv1.AddCommentRequest) (*taskv1.AddCommentResponse, error) {
			gotReq = in
			return &taskv1.AddCommentResponse{Id: "c1", Content: in.GetContent()}, nil
		},
	}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.addComment", argsJSON(t, map[string]any{"taskId": "t1", "content": "hello"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTaskId() != "t1" || gotReq.GetContent() != "hello" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*taskv1.AddCommentResponse)
	if !ok || resp.GetId() != "c1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskChannels_ListComments(t *testing.T) {
	var gotReq *taskv1.ListCommentsRequest
	fake := &fakeTaskSubtreeClient{
		listCommentsFunc: func(ctx context.Context, in *taskv1.ListCommentsRequest) (*taskv1.ListCommentsResponse, error) {
			gotReq = in
			return &taskv1.ListCommentsResponse{Comments: []*taskv1.AddCommentResponse{{Id: "c1"}}, NextPageToken: "next"}, nil
		},
	}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.listComments", argsJSON(t, map[string]any{"taskId": "t1", "pageToken": "p", "pageSize": 10}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTaskId() != "t1" || gotReq.GetPageToken() != "p" || gotReq.GetPageSize() != 10 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*taskv1.ListCommentsResponse)
	if !ok || len(resp.GetComments()) != 1 || resp.GetNextPageToken() != "next" {
		t.Errorf("unexpected result: %+v", result)
	}
}
