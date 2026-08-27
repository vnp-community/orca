# TASK-TG-04-02: `WorktreeProvisioner` adapter — reuse-or-create via `git-gateway-service.CreateWorktree`

**From Solution:** SOL-TG-04
**Priority:** P1
**Service:** `task-service` (client) + `git-gateway-service` (existing `CreateWorktree` RPC, reused as-is)
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go` (new)
**Depends on:** TASK-TG-01-01 (`Task.WorktreeID` field)
**Status:** `[ ]` TODO

---

## Context

Spec: "IF task.worktreeId exists: use existing worktree ELSE: create one."
`git-gateway-service`'s `CreateWorktree` RPC (`gitgateway.proto:604-627`)
already does "`git worktree add`" + `project-service` bookkeeping in one
saga (`git-gateway-service/internal/usecase/create_worktree.go`) — this
adapter is a thin caller, not a re-implementation of that saga.

## Changes to make

Add a `WorktreeProvisioner` port to
`backend-go/services/task-service/internal/usecase/ports.go`:

```go
// WorktreeProvisioner implements Execute's "reuse or create" worktree step
// — a task with an existing WorktreeID reuses it; otherwise a new one is
// created via git-gateway-service's existing CreateWorktree saga.
type WorktreeProvisioner interface {
	EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error)
}
```

Create `backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go`:

```go
package grpcclient

import (
	"context"
	"fmt"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// WorktreeProvisioner implements usecase.WorktreeProvisioner against
// git-gateway-service's CreateWorktree RPC — delegates the whole
// create+record saga rather than re-implementing it.
type WorktreeProvisioner struct {
	git gitgatewayv1.GitGatewayServiceClient
}

func NewWorktreeProvisioner(git gitgatewayv1.GitGatewayServiceClient) *WorktreeProvisioner {
	return &WorktreeProvisioner{git: git}
}

func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error) {
	if task.WorktreeID != "" {
		return task.WorktreeID, "", nil // reuse — spec's "IF task.worktreeId exists: use existing worktree". Caller resolves the path separately via ProjectExecutionResolver, unchanged from today.
	}
	resp, err := p.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: task.ProjectID,
		Branch:    fmt.Sprintf("task/%s", task.ID),
		TaskId:    &task.ID,
	})
	if err != nil {
		return "", "", fmt.Errorf("worktree_provisioner: create worktree: %w", err)
	}
	return resp.GetWorktreeId(), resp.GetPath(), nil
}
```

**Open wiring detail, flagged rather than guessed at**: `CreateWorktreeRequest`
requires a `repo_id` field (`gitgateway.proto:606`) this adapter's call
above does NOT set — `task.ProjectID` alone doesn't resolve one. Confirm at
implementation time which of these two is correct for the current
`project-service` repo-per-project cardinality:

1. `CreateWorktreeRequest` gains a `project_id`-based resolution path
   server-side in `git-gateway-service` (it already calls `project-service`
   in the same saga, so this is a natural place for it), or
2. `WorktreeProvisioner` resolves `RepoID` itself via a `project-service`
   call first (would need a new port here, e.g. `RepoResolver`).

Do not guess — read `git-gateway-service/internal/usecase/create_worktree.go`'s
current `CreateWorktreeInput` shape and `project-service`'s
project→repo cardinality before choosing, and update this file's snippet
accordingly.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestWorktreeProvisioner -v
```

Expected: `worktree_provisioner_test.go` — a task with an existing
`WorktreeID` never calls `CreateWorktree`; a task with an empty `WorktreeID`
does, and the returned ID/path match `CreateWorktreeResponse`.
