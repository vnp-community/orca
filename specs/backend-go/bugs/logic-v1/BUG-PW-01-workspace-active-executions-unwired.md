# BUG-PW-01: Workspace switch cannot load "active workflows" — no `workflow.*`/`task.*` active-executions channel wired

**Business Logic:** [BL-PW-01](../../../../docs/logic/project-workspace/BL-PW-01-workspace-context.md) — Project Workspace Context
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** When a developer switches to a project, the parallel workspace-load step that is supposed to fetch `activeWorkflows` for the new WorkspaceContext has no backend-go channel to call — the frontend can load git status, worktrees, and the file tree root for real, but has no way to ask "does this project have any workflow execution running right now" over the WS-compat surface, even though the underlying gRPC capability (`HasActiveExecutions`) is fully built and working in both `workflow-service` and `task-service`.

---

## Spec summary

BL-PW-01 defines `WorkspaceContext`, the central state object every workspace panel reads from. On "Switch Project" the spec's flow loads four things in parallel — `git.status`, `git.worktree.list`, `fs.readDir` (root), and `WorkflowService.getActiveExecutions(projectId)` — then resolves the user's profile and starts background polls (git status every 5s, server health every 30s). `activeWorkflowExecutionIds` is a first-class field on `WorkspaceContext` (doc line 46) and is read by the Tasks/Workflow panels.

## What backend-go has

The other three legs of the parallel load, plus profile resolution, are wired for real:

- `git.status` → `GitGatewayServiceClient.GetStatus` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:267-281`
- `worktree.list` → `ProjectServiceClient.ListWorktrees` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:159-174`
- `workspace.refreshFileTree` (the `fs.readDir`-equivalent used by `WorkspaceContext.tsx`/`WorkspaceContextV6.tsx`) → resolves the project's active worktree then calls `GitGatewayServiceClient.ReadDir` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:946-975`
- `profile.getResolved` → `TenantServiceClient.GetResolvedProfile` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:52-61`
- `project.get` → `ProjectServiceClient.GetProject` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_tenant_project.go:198-215`

The "active executions" primitive itself is genuinely implemented, not just stubbed:

- proto: `rpc HasActiveExecutions(HasActiveExecutionsRequest) returns (HasActiveExecutionsResponse)` — `backend-go/proto/orca/workflow/v1/workflow.proto:50` (and mirrored on task-service: `backend-go/proto/orca/task/v1/task.proto:29`)
- usecase: `backend-go/services/workflow-service/internal/usecase/has_active_executions.go:15-38` — queries `ExecutionRepository.HasActiveExecutions(ctx, tenantID, projectID)`, backed by real Postgres at `backend-go/services/workflow-service/internal/adapter/postgres/repository.go:216-219+`
- gRPC server: `backend-go/services/workflow-service/internal/adapter/grpc/server.go:125-130`

## What's missing

- No `workflow.*` (or `task.*`) channel exposes `HasActiveExecutions` through `wscompat`. `registerWorkflowChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go:21,43,59,81`) registers only `workflow.execute`, `workflow.cancel`, `workflow.template.create`, `workflow.template.update` — confirmed via `grep -rn 'hasActiveExecutions\|HasActiveExecutions' backend-go/services/api-gateway/internal/adapter/wscompat/*.go` returning zero non-test matches.
- Result: the frontend's `Promise.all([...])` parallel workspace load has no real 4th leg — `activeWorkflowExecutionIds` can only ever be populated by a client-side placeholder/empty value, not a real backend answer, even though `workflow-service` already knows the answer.
- `RelayConnectionPool` (singleton per dev server, idle-cleanup after 5 min) and `FleetHealthMonitor.getCached()`'s TTL-cached offline-mode banner are process/runtime concerns that live outside backend-go's Go microservices (dev-server relay/session management), so they are out of scope for this report — not counted as a gap here.

## See also

- No prior missing-v1/api-v1 bug covers this specific gap; it is adjacent to `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` (workflow.* namespace gaps) but that report predates `workflow.execute`/`workflow.cancel` landing, so re-check its status before treating it as current.

## References

- `docs/logic/project-workspace/BL-PW-01-workspace-context.md:79-96` — the "Load workspace data (parallel)" step naming `WorkflowService.getActiveExecutions(projectId)`
- `backend-go/proto/orca/workflow/v1/workflow.proto:50` — `HasActiveExecutions` RPC definition
- `backend-go/services/workflow-service/internal/usecase/has_active_executions.go:15-38` — real usecase
- `backend-go/services/workflow-service/internal/adapter/grpc/server.go:125-130` — real gRPC server method
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go:21,43,59,81` — everything currently registered under `workflow.*` (no active-executions channel)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:267-281` (`git.status`), `channels_worktree.go:159-174` (`worktree.list`), `channels_git.go:946-975` (`workspace.refreshFileTree`), `channels_tenant_project.go:52-61,198-215` (`profile.getResolved`, `project.get`) — the three legs that ARE wired for real
