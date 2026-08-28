# TASK-PRF-04-02: Implement `GetProjectContext` usecase + gRPC handler in `project-service`

**From Solution:** SOL-PRF-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/get_project_context.go`
**Depends on:** TASK-PRF-04-01
**Status:** `[x]` DONE — GetProjectContext usecase + grpc handler + main.go wiring (InfraFleetHostnameResolver adapter) added; membership-gated, best-effort hostname; tests pass

---

## Context

Implements the RPC TASK-PRF-04-01 added to the proto. Access control reuses
the existing any-member gate (`projectActionAnyMember` /
`requireProjectAccess`) unchanged — an execution-dispatch caller
(`workflow-service`/`task-service`, acting on behalf of an already-
authenticated end user) must present that user's membership, not a
service-identity bypass, so no new authorization branch is needed.

## Changes to make

Add a `DevServerHostnameResolver` port to
`backend-go/services/project-service/internal/usecase/ports.go`:

```go
// DevServerHostnameResolver resolves a dev server id to its host string via
// infra-fleet-service.ListDevServers — best-effort, used only by
// GetProjectContext's display-only dev_server_hostname field. A lookup
// failure never fails the whole GetProjectContext read.
type DevServerHostnameResolver interface {
	Hostname(ctx context.Context, tenantID, devServerID string) (string, error)
}
```

Create `backend-go/services/project-service/internal/usecase/get_project_context.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type GetProjectContext struct {
	projects ProjectRepository
	repos    RepoRepository
	hosts    DevServerHostnameResolver
	opa      OPAClient
}

func NewGetProjectContext(projects ProjectRepository, repos RepoRepository, hosts DevServerHostnameResolver, opa OPAClient) *GetProjectContext {
	return &GetProjectContext{projects: projects, repos: repos, hosts: hosts, opa: opa}
}

func (uc *GetProjectContext) Execute(ctx context.Context, projectID string) (domain.ProjectContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectContext{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.projects, uc.opa, projectID, projectActionAnyMember); err != nil {
		return domain.ProjectContext{}, err
	}

	project, err := uc.projects.Get(ctx, tenantID, projectID)
	if err != nil {
		return domain.ProjectContext{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", err)
	}

	repos, _ := uc.repos.ListRepos(ctx, projectID) // best-effort; a project with no repos yet has an empty RepoURL
	var repoURL string
	if len(repos) > 0 {
		repoURL = repos[0].URL
	}
	hostname, _ := uc.hosts.Hostname(ctx, tenantID, project.DevServerID) // "" on any failure — display-only field, never fails the read

	return domain.ProjectContext{
		ProjectID: project.ID, ProjectName: project.Name, Description: project.Description,
		RepoURL: repoURL, DevServerID: project.DevServerID, DevServerHostname: hostname,
	}, nil
}
```

Add `domain.ProjectContext` to
`backend-go/services/project-service/internal/domain/project.go`:

```go
// ProjectContext is GetProjectContext's read-only view — a subset of
// Project plus a best-effort-resolved dev server hostname, per
// project-service.md §2's Boundary decision.
type ProjectContext struct {
	ProjectID, ProjectName, Description string
	RepoURL, DevServerID, DevServerHostname string
}
```

Add the gRPC handler to
`backend-go/services/project-service/internal/adapter/grpc/server.go`,
matching this file's existing handler shape (`toProtoProject`-style
converter, `apperrors.ToGRPCStatus`):

```go
func (s *Server) GetProjectContext(ctx context.Context, req *projectv1.GetProjectContextRequest) (*projectv1.ProjectContext, error) {
	pc, err := s.getProjectContext.Execute(ctx, req.GetProjectId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.ProjectContext{
		ProjectId: pc.ProjectID, ProjectName: pc.ProjectName, Description: pc.Description,
		RepoUrl: pc.RepoURL, DevServerId: pc.DevServerID, DevServerHostname: pc.DevServerHostname,
	}, nil
}
```

Wire `getProjectContextUC := usecase.NewGetProjectContext(repo, repoRepo,
hostnameResolver, opa)` and add it to the `New(...)` server constructor call
in `cmd/server/main.go`, alongside the existing usecase wiring — a
`DevServerHostnameResolver` implementation (thin wrapper over
`infra-fleet-service.ListDevServers`, same dial as `InfraFleetDevServerLister`)
should be added in this same task.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go test ./services/project-service/... -run GetProjectContext -v
```

Add `get_project_context_test.go` per SOL-PRF-04's Test plan:
membership-gated (non-member denied via the existing
`projectActionAnyMember` OPA path); `RepoURL` empty when the project has zero
repos (not an error); `DevServerHostname` empty when the hostname resolver
errors (best-effort, never fails the whole RPC).
