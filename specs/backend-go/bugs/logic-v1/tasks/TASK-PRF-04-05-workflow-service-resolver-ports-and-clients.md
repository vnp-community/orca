# TASK-PRF-04-05: Add `ProfileResolver`/`ProjectContextResolver` ports and gRPC clients to `workflow-service`

**From Solution:** SOL-PRF-04
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/ports.go`
**Depends on:** TASK-PRF-04-01
**Status:** `[x]` DONE — ProfileResolver/ProjectContextResolver ports + infrafleetclient adapters added, both forwarding tenant/user identity via outbound gRPC metadata (deviates from literal spec sketch — required since tenant-service.GetResolvedProfile and project-service.GetProjectContext both call tenant.RequireTenantID/UserID server-side); TenantServiceAddr/ProjectServiceAddr config added; go build clean

---

## Context

`AgentExecutor` (TASK-PRF-04-06) needs to resolve the acting user's
`ResolvedProfile` (from `tenant-service`) and the step's project context
(from `project-service`, via TASK-PRF-04-01's new RPC) before it can build
env/preamble. Neither client is dialed by `workflow-service` today —
`02-microservices-decomposition.md`'s dependency graph shows `task -->
tenant` but is flagged as **missing** the `wf --> tenant` edge this task
adds in practice (the prose in `tenant-service.md` §7 already asserts the
edge as intended, this closes the gap between prose and graph/code).

## Changes to make

Add to `backend-go/services/workflow-service/internal/usecase/ports.go`:

```go
// ProfileResolver is the outbound port toward tenant-service.GetResolvedProfile
// — a NEW dependency edge (tenant-service.md §7 already documents this as
// intended for task-service/workflow-service; workflow-service never
// exercised it before this task). Returns the already-JSON-decoded
// resolved_settings_json as a generic map, matching the shape
// domain.BuildAgentEnv reads.
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error)
}

// ProjectContextResolver is the outbound port toward
// project-service.GetProjectContext (TASK-PRF-04-01/02).
type ProjectContextResolver interface {
	GetProjectContext(ctx context.Context, projectID string) (ProjectContext, error)
}

// ProjectContext is the subset of project-service's ProjectContext this
// service's agent-spawn preamble needs — decoded from the gRPC response by
// internal/adapter/infrafleetclient's ProjectContextResolver implementation.
type ProjectContext struct {
	ProjectID, ProjectName, Description string
	RepoURL, DevServerID, DevServerHostname string
}
```

Create `backend-go/services/workflow-service/internal/adapter/infrafleetclient/profile_resolver.go`:

```go
package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// ProfileResolver implements usecase.ProfileResolver by dialing
// tenant-service's GetResolvedProfile RPC directly (not through
// infra-fleet-service's Relay — this is a normal service-to-service gRPC
// call, unrelated to the Dev-Server-Agent relay this package's other files
// use). Kept in this package for now to avoid a third adapter subpackage;
// revisit if workflow-service grows more tenant-service call sites.
type ProfileResolver struct {
	client tenantv1.TenantServiceClient
}

func NewProfileResolver(client tenantv1.TenantServiceClient) *ProfileResolver {
	return &ProfileResolver{client: client}
}

func (r *ProfileResolver) GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error) {
	resp, err := r.client.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("infrafleetclient: tenant-service GetResolvedProfile: %w", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &settings); err != nil {
		return nil, fmt.Errorf("infrafleetclient: unmarshal resolved_settings_json: %w", err)
	}
	return settings, nil
}
```

Create `backend-go/services/workflow-service/internal/adapter/infrafleetclient/project_context_resolver.go`:

```go
package infrafleetclient

import (
	"context"
	"fmt"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// ProjectContextResolver implements usecase.ProjectContextResolver by
// dialing project-service's GetProjectContext RPC directly.
type ProjectContextResolver struct {
	client projectv1.ProjectServiceClient
}

func NewProjectContextResolver(client projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{client: client}
}

func (r *ProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	resp, err := r.client.GetProjectContext(ctx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("infrafleetclient: project-service GetProjectContext: %w", err)
	}
	return usecase.ProjectContext{
		ProjectID: resp.GetProjectId(), ProjectName: resp.GetProjectName(), Description: resp.GetDescription(),
		RepoURL: resp.GetRepoUrl(), DevServerID: resp.GetDevServerId(), DevServerHostname: resp.GetDevServerHostname(),
	}, nil
}
```

Add `TenantServiceAddr`/`ProjectServiceAddr` config fields to
`backend-go/services/workflow-service/internal/config/config.go`, matching
the existing `InfraFleetServiceAddr` naming convention, and dial both new
clients in `cmd/server/main.go` (this is completed alongside
TASK-PRF-04-06's executor wiring, since that's where the constructed
resolvers are actually consumed).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/internal/usecase/... ./services/workflow-service/internal/adapter/infrafleetclient/...
```

Full build/test for `cmd/server` lands with TASK-PRF-04-06 once `AgentExecutor`
actually consumes these ports.
