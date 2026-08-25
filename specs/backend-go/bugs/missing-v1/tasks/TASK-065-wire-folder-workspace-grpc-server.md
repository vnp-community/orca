# TASK-065: Wire `FolderWorkspace` RPCs into `project-service`'s gRPC server and composition root

**From Solution:** SOL-010 (implied by the proto/usecase split, same shape TASK-056 uses for `git-gateway-service`)
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/adapter/grpc/server.go`, `backend-go/services/project-service/cmd/server/main.go`
**Depends on:** TASK-061, TASK-064
**Status:** `[x]` DONE (verified) — `Server`/`Deps`/`New` extended,
5 translation methods + `toProtoFolderWorkspace` added, composition root
in `cmd/server/main.go` wired (`folderWorkspaceRepo`/`folderWorkspaceUC`).
`go build ./...` and `go vet ./...` clean across the whole service.

---

## Context

`project-service`'s gRPC `Server` implements `ProjectServiceServer` by
translating each RPC to a usecase call — same shape
`git-gateway-service`'s `Server` uses (see TASK-056). The 5 new
`FolderWorkspace` RPCs need the same treatment, or they fall through to
`UnimplementedProjectServiceServer`'s default `Unimplemented` response.

---

## Changes to make

### Step 1: Extend `Server` struct and `New`

**File:** `internal/adapter/grpc/server.go`

Add a `folderWorkspaces *usecase.FolderWorkspaceUseCase` field to the
existing `Server` struct and constructor parameter to `New(...)`,
following the same append pattern as this service's other usecase fields.

### Step 2: Add translation methods

```go
func (s *Server) CreateFolderWorkspace(ctx context.Context, req *projectv1.CreateFolderWorkspaceRequest) (*projectv1.FolderWorkspace, error) {
	id := identityFromContext(ctx) // reuse this server's existing identity-extraction helper
	fw, err := s.folderWorkspaces.Create(ctx, id, usecase.CreateFolderWorkspaceInput{
		DevServerID: req.GetDevServerId(), Path: req.GetPath(), Name: req.GetName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoFolderWorkspace(fw), nil
}

func (s *Server) UpdateFolderWorkspace(ctx context.Context, req *projectv1.UpdateFolderWorkspaceRequest) (*projectv1.FolderWorkspace, error) {
	id := identityFromContext(ctx)
	fw, err := s.folderWorkspaces.Update(ctx, id, req.GetId(), req.GetName())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoFolderWorkspace(fw), nil
}

func (s *Server) DeleteFolderWorkspace(ctx context.Context, req *projectv1.DeleteFolderWorkspaceRequest) (*emptypb.Empty, error) {
	id := identityFromContext(ctx)
	if err := s.folderWorkspaces.Delete(ctx, id, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListFolderWorkspaces(ctx context.Context, req *projectv1.ListFolderWorkspacesRequest) (*projectv1.ListFolderWorkspacesResponse, error) {
	id := identityFromContext(ctx)
	list, err := s.folderWorkspaces.List(ctx, id)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.FolderWorkspace, 0, len(list))
	for _, fw := range list {
		out = append(out, toProtoFolderWorkspace(fw))
	}
	return &projectv1.ListFolderWorkspacesResponse{FolderWorkspaces: out}, nil
}

func (s *Server) GetFolderWorkspacePathStatus(ctx context.Context, req *projectv1.GetFolderWorkspacePathStatusRequest) (*projectv1.GetFolderWorkspacePathStatusResponse, error) {
	id := identityFromContext(ctx)
	result, err := s.folderWorkspaces.GetPathStatus(ctx, id, req.GetDevServerId(), req.GetPath())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.GetFolderWorkspacePathStatusResponse{
		Status:                    result.Status,
		ExistingFolderWorkspaceId: result.ExistingID,
	}, nil
}

func toProtoFolderWorkspace(fw domain.FolderWorkspace) *projectv1.FolderWorkspace {
	return &projectv1.FolderWorkspace{
		Id: fw.ID, DevServerId: fw.DevServerID, Path: fw.Path, Name: fw.Name,
		AddedBy: fw.AddedBy, CreatedAt: timestamppb.New(fw.CreatedAt),
	}
}
```

Reuse this service's existing identity-from-context helper (find the
function `GetStatus`/`CreateProject`'s translation methods already call
for tenant/user extraction — do not introduce a second one) and its
existing `toProtoX` naming convention for the response mapper. Add
`"google.golang.org/protobuf/types/known/emptypb"` and
`"google.golang.org/protobuf/types/known/timestamppb"` to the import
block if not already present.

### Step 3: Extend `cmd/server/main.go`'s composition root

```go
folderWorkspaceRepo := postgres.NewFolderWorkspaceRepository(pool) // reuse the existing pool variable
folderWorkspaceUseCase := usecase.NewFolderWorkspaceUseCase(folderWorkspaceRepo)

server := grpc.New(
	// ...existing usecase args...,
	folderWorkspaceUseCase,
)
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./...
go vet ./...
```
