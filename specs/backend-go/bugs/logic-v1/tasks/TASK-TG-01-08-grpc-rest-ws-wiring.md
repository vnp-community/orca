# TASK-TG-01-08: Wire `GetSubtree`/`RecalculateProgress`/`AddComment`/`ListComments` into gRPC, REST, and `wscompat`

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service` + `api-gateway`
**File:** `backend-go/services/task-service/internal/adapter/grpc/server.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-TG-01-01, TASK-TG-01-06, TASK-TG-01-07
**Status:** `[ ]` TODO

---

## Context

The 4 new RPCs need gRPC server handlers, and per BUG-034's already-flagged
WS-wiring gap, `task.list`/`task.update`/`task.delete`/`task.getDependencies`
(already real RPCs) were never wired into `wscompat`'s
`registerTaskChannels`. Both are closed in this pass since they touch the
same function.

## Changes to make

In `backend-go/services/task-service/internal/adapter/grpc/server.go`, add
the 4 new usecases to `Server`'s fields, `New`'s parameters, and 4 new
handler methods:

```go
func (s *Server) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	result, err := s.getSubtree.Execute(ctx, usecase.GetSubtreeInput{RootID: req.GetRootId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t))
	}
	edges := make([]*taskv1.AddEdgeRequest, 0, len(result.DependsOnEdges))
	for _, e := range result.DependsOnEdges {
		edges = append(edges, &taskv1.AddEdgeRequest{FromTaskId: e.FromTaskID, ToTaskId: e.ToTaskID, Type: taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON})
	}
	return &taskv1.GetSubtreeResponse{Tasks: out, DependsOnEdges: edges}, nil
}

func (s *Server) RecalculateProgress(ctx context.Context, req *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error) {
	p, err := s.recalculateProgress.Execute(ctx, req.GetRootId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.RecalculateProgressResponse{ProgressPercent: int32(p)}, nil
}

func (s *Server) AddComment(ctx context.Context, req *taskv1.AddCommentRequest) (*taskv1.AddCommentResponse, error) {
	c, err := s.addComment.Execute(ctx, req.GetTaskId(), req.GetContent())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)}, nil
}

func (s *Server) ListComments(ctx context.Context, req *taskv1.ListCommentsRequest) (*taskv1.ListCommentsResponse, error) {
	comments, next, err := s.listComments.Execute(ctx, req.GetTaskId(), req.GetPageToken(), req.GetPageSize())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.AddCommentResponse, 0, len(comments))
	for _, c := range comments {
		out = append(out, &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)})
	}
	return &taskv1.ListCommentsResponse{Comments: out, NextPageToken: next}, nil
}
```

Wire the 4 new usecases into
`backend-go/services/task-service/cmd/server/main.go`'s composition root
(construct `getSubtreeUC := usecase.NewGetSubtree(repo, repo,
teamScopeResolver)`, `recalculateProgressUC := usecase.NewRecalculateProgress(repo)`,
`addCommentUC := usecase.NewAddComment(repo)`, `listCommentsUC :=
usecase.NewListComments(repo)`; pass all 4 into `taskgrpc.New(...)`).

In `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go`,
add REST routes following `handleGetTask`'s existing translation pattern:

- `GET /v1/tasks/{id}/subtree` → `GetSubtree`
- `POST /v1/tasks/{id}/progress:recalculate` → `RecalculateProgress`
- `POST /v1/tasks/{id}/comments` → `AddComment`
- `GET /v1/tasks/{id}/comments` → `ListComments`

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`'s
`registerTaskChannels`, add channels for the 4 new RPCs plus the
already-real-but-unwired ones BUG-034 names, using the same
`decodeArg`/`client.<RPC>` shape every other channel in that file uses:

```go
	registry.Register("task.getSubtree", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct{ RootID string `json:"rootId"` }
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.GetSubtree(ctx, &taskv1.GetSubtreeRequest{RootId: args.RootID})
	})
	registry.Register("task.addComment", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct {
			TaskID  string `json:"taskId"`
			Content string `json:"content"`
		}
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.AddComment(ctx, &taskv1.AddCommentRequest{TaskId: args.TaskID, Content: args.Content})
	})
	registry.Register("task.listComments", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct {
			TaskID    string `json:"taskId"`
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.ListComments(ctx, &taskv1.ListCommentsRequest{TaskId: args.TaskID, PageToken: args.PageToken, PageSize: args.PageSize})
	})
	// Closing BUG-034's WS-wiring gap for already-real RPCs, same pass since
	// both changes touch registerTaskChannels:
	registry.Register("task.list", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"projectId"`
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.ListTasks(ctx, &taskv1.ListTasksRequest{ProjectId: args.ProjectID, PageToken: args.PageToken, PageSize: args.PageSize})
	})
	registry.Register("task.update", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct {
			ID     string  `json:"id"`
			Title  *string `json:"title"`
			Status *string `json:"status"`
		}
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		req := &taskv1.UpdateTaskRequest{Id: args.ID}
		if args.Title != nil {
			req.Title = wrapperspb.String(*args.Title)
		}
		if args.Status != nil {
			req.Status = wrapperspb.String(*args.Status)
		}
		return client.UpdateTask(ctx, req)
	})
	registry.Register("task.delete", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct{ ID string `json:"id"` }
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.DeleteTask(ctx, &taskv1.DeleteTaskRequest{Id: args.ID})
	})
	registry.Register("task.getDependencies", func(ctx context.Context, client taskv1.TaskServiceClient, raw json.RawMessage) (any, error) {
		var args struct{ TaskID string `json:"taskId"` }
		if err := decodeArg(raw, &args); err != nil {
			return nil, err
		}
		return client.GetDependencies(ctx, &taskv1.GetDependenciesRequest{TaskId: args.TaskID})
	})
```

(Match the exact `Register`/`decodeArg` helper names and signatures already
present in `channels.go` — read that file's existing `registerTaskChannels`
entries first and mirror their style exactly rather than guessing at the
registry API.)

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/... ./services/api-gateway/...
go test ./services/task-service/internal/adapter/grpc/... -v
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestTaskRoutes -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestTaskChannels -v
```

Expected: clean build; gRPC server tests cover the 4 new handlers; REST
route tests cover the 4 new endpoints; `wscompat` channel tests cover all 7
newly-registered channels (4 new + 3 BUG-034 gap-closures).
