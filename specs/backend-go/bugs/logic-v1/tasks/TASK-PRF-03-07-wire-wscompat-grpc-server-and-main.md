# TASK-PRF-03-07: Wire `devServerId`/`repoPath` through wscompat, gRPC handler, and `cmd/server/main.go`

**From Solution:** SOL-PRF-03
**Priority:** P1 — closes the loop end-to-end; the package doesn't build until this lands
**Service:** `project-service` + `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go`
**Depends on:** TASK-PRF-03-04, TASK-PRF-03-05, TASK-PRF-03-06
**Status:** `[x]` DONE — not in original task list but implemented anyway to keep the workspace build green (PRF-03-04/05/06 changed constructor signatures main.go must compile against); wscompat devServerId/repoPath threading + grpc handler + main.go wiring all done; go build ./... clean, wscompat regression test added

---

## Context

`wscompat`'s `project.create` already decodes `devServerId` off the wire but
silently drops it — the `CreateProjectRequest` it builds never sets the
field, because the proto field didn't exist until TASK-PRF-03-01. This task
fixes that wiring, adds `repoPath` decoding (never decoded at all before),
threads both through `project-service`'s gRPC handler, and updates every
constructor call TASK-PRF-03-04/05/06 changed in `cmd/server/main.go`.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go`
(around line 173's `project.create` handler):

```go
r.Register("project.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type createArgs struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		DevServerID   string `json:"devServerId"`
		RepoPath      string `json:"repoPath"` // NEW — wasn't decoded at all before
		DefaultBranch string `json:"defaultBranch"`
		Visibility    string `json:"visibility"`
	}
	in, err := decodeArg[createArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := client.CreateProject(rpcCtx, &projectv1.CreateProjectRequest{
		TenantId: id.TenantID, Name: in.Name, Description: in.Description,
		DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
		DevServerId: in.DevServerID, RepoPath: in.RepoPath, // NEW
	})
	if err != nil {
		return nil, err
	}
	return resp.GetProject(), nil
})
```

In `backend-go/services/project-service/internal/adapter/grpc/server.go`'s
`CreateProject` handler:

```go
func (s *Server) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error) {
	project, err := s.createProject.Execute(ctx, usecase.CreateProjectInput{
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		DefaultBranch: req.GetDefaultBranch(),
		Visibility:    req.GetVisibility(),
		DevServerID:   req.GetDevServerId(), // NEW
		RepoPath:      req.GetRepoPath(),    // NEW
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateProjectResponse{Project: toProtoProject(project)}, nil
}
```

In `backend-go/services/project-service/cmd/server/main.go`, update the
three constructor calls TASK-PRF-03-04/05/06 changed the signature of, and
construct the new adapters (dial the shared `*grpc.ClientConn` to
infra-fleet-service that `InfraFleetDevServerLister` already dials, plus a
new dial to `tenant-service` for `ProfileResolver`, plus construct the new
`adapter/eventbus.Publisher` off this service's NATS connection — follow
`tenant-service/cmd/server/main.go`'s `eventbus.Connect`/`EnsureStream`
pattern for a best-effort, non-fatal-if-NATS-unreachable NATS block, since
`project-service` has none today):

```go
healthChecker := grpcclient.NewInfraFleetHealthChecker(infraFleetClient)
profileResolver := grpcclient.NewProfileResolver(tenantClient, infraFleetClient)
// ... eventbus.Connect / EnsureStream / projecteventbus.New(pub) -> auditPublisher, memberNotifier ...

createProjectUC := usecase.NewCreateProject(repo, repoRepo, devServerLister, healthChecker, devServerRelay)
listProjectsUC := usecase.NewListProjects(repo, profileResolver)
rebindDevServerUC := usecase.NewRebindDevServer(repo, workflowChecker, taskChecker, opa, devServerLister, healthChecker, auditPublisher, memberNotifier)
```

Add a `TENANT_SERVICE_ADDR` config field to `project-service`'s
`internal/config/config.go` for the new tenant-service dial, matching the
`*_ADDR` naming convention already used for `INFRA_FLEET_SERVICE_ADDR` etc.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go vet ./services/project-service/... ./services/api-gateway/...
```

Run every test case queued by TASK-PRF-03-02/03/04/05/06's Verify sections
now that the package compiles end-to-end:

```bash
go test ./services/project-service/... -v
go test ./services/api-gateway/internal/adapter/wscompat/... -v
```
