# TASK-PRF-04-07: Add `ProfileResolver`/`ProjectContextResolver` ports and gRPC clients to `task-service`

**From Solution:** SOL-PRF-04
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/ports.go`
**Depends on:** TASK-PRF-04-01
**Status:** `[x]` DONE — ProfileResolver/ProjectContextResolver ports + grpcclient adapters added (same identity-forwarding approach as workflow-service); TenantServiceAddr/ProjectServiceAddr config added; go build clean

---

## Context

Same two ports as TASK-PRF-04-05, `task-service`'s own copies. Confirmed
from `task-service.md`'s dependency table: `task --> tenant` already exists
(`TeamScopeResolver` dials tenant-service today, even though its own
implementation is stubbed per `ports.go`'s doc comment) — only the
`project-service` client dial for `ProjectContextResolver` is genuinely new.
The `ProfileResolver` port itself is new (no usecase in `task-service` calls
`GetResolvedProfile` today), but the underlying gRPC channel to
`tenant-service` is not.

## Changes to make

Add to `backend-go/services/task-service/internal/usecase/ports.go`:

```go
// ProfileResolver is the outbound port toward tenant-service.GetResolvedProfile
// — task-service.md §7 already lists a task --> tenant edge (TeamScopeResolver
// dials it, albeit stubbed); this is a second, independent use of that same
// dependency, not a new service edge.
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error)
}

// ProjectContextResolver is the outbound port toward
// project-service.GetProjectContext (TASK-PRF-04-01/02) — a NEW dial
// (task-service has no project-service client today; ProjectExecutionResolver
// goes through infra-fleet-service instead, see its own doc comment).
type ProjectContextResolver interface {
	GetProjectContext(ctx context.Context, projectID string) (ProjectContext, error)
}

// ProjectContext mirrors workflow-service's own copy of this struct —
// deliberate per-service duplication, same rationale as
// domain/agent_environment.go's (TASK-PRF-04-04).
type ProjectContext struct {
	ProjectID, ProjectName, Description string
	RepoURL, DevServerID, DevServerHostname string
}
```

Create `backend-go/services/task-service/internal/adapter/grpcclient/profile_resolver.go`
and `project_context_resolver.go`, structurally identical to
TASK-PRF-04-05's `workflow-service` versions (same RPC calls, same
JSON-decode of `resolved_settings_json`), adjusted only for
`task-service`'s `package grpcclient` and import paths:

```go
package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

type ProfileResolver struct {
	client tenantv1.TenantServiceClient
}

func NewProfileResolver(client tenantv1.TenantServiceClient) *ProfileResolver {
	return &ProfileResolver{client: client}
}

func (r *ProfileResolver) GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error) {
	resp, err := r.client.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &settings); err != nil {
		return nil, fmt.Errorf("grpcclient: unmarshal resolved_settings_json: %w", err)
	}
	return settings, nil
}
```

```go
package grpcclient

import (
	"context"
	"fmt"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

type ProjectContextResolver struct {
	client projectv1.ProjectServiceClient
}

func NewProjectContextResolver(client projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{client: client}
}

func (r *ProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	resp, err := r.client.GetProjectContext(ctx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("grpcclient: project-service GetProjectContext: %w", err)
	}
	return usecase.ProjectContext{
		ProjectID: resp.GetProjectId(), ProjectName: resp.GetProjectName(), Description: resp.GetDescription(),
		RepoURL: resp.GetRepoUrl(), DevServerID: resp.GetDevServerId(), DevServerHostname: resp.GetDevServerHostname(),
	}, nil
}
```

Add a `ProjectServiceAddr` config field to
`backend-go/services/task-service/internal/config/config.go` (confirm
whether a `TenantServiceAddr` already exists for `TeamScopeResolver`'s dial
— reuse it if so, add if not) and dial both clients in `cmd/server/main.go`
— completed alongside TASK-PRF-04-08's executor wiring.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/internal/usecase/... ./services/task-service/internal/adapter/grpcclient/...
```

Full build/test for `cmd/server` lands with TASK-PRF-04-08 once
`SimpleExecutor` actually consumes these ports.
