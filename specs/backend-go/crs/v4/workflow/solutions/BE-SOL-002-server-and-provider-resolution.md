# BE-SOL-002: Server target resolution (`project:`/`server:`/`fleet:tag:`) + AI provider resolution

**Resolves:** [CR-WF-002](../../../../../../docs/crs/v4/workflow/CR-WF-002-server-and-provider-resolution.md)
**Service:** `workflow-service` only (calls out to `project-service`/`infra-fleet-service`/`ai-provider-service`, no changes proposed to those services beyond confirming their RPCs exist — see verification note)
**Depends on:** [BE-SOL-001](./BE-SOL-001-fix-agent-step-executor-relay-method.md)
**Affected files (proposed):**
- `backend-go/services/workflow-service/internal/domain/target_spec.go` (new)
- `backend-go/services/workflow-service/internal/usecase/resolve_target.go`, `resolve_provider.go` (new)
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `shell_step_executor.go`, `notification_step_executor.go` (call the new resolvers instead of reading `ConnectionID` as a bare literal)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** §"Design — ProviderResolver"
> gọi 1 RPC tên `ResolveForContext` không tồn tại — RPC thật là
> **`ResolveProvider`** (`ResolveProviderRequest{tenant_id, user_id,
> project_id}` → `ResolveProviderResponse{account: ProviderAccount}`,
> `ProviderAccount` không có field `Model`). Xem
> [TASK-WF-002-02](../tasks/TASK-WF-002-02-provider-resolver.md) cho shape
> đã sửa. Đồng thời, §"verify before implementing" về `PickByTag` đã được
> xác nhận **là blocker thật** (không phải nghi ngờ) — `infra-fleet-service`
> chưa có RPC này lẫn usecase tương đương nào để tái dùng; xem
> [TASK-WF-002-04](../tasks/TASK-WF-002-04-infra-fleet-pick-by-tag-rpc.md)
> (task mới, không thuộc solution này) để lấp gap trước khi implement
> nhánh `fleet:tag:`.

---

## Current state

`AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig` all carry a
bare `ConnectionID string` (`step.go:58-87`) whose own doc comment already
names this exact gap: *"nothing in this scaffold previously identified
*which* infra-fleet-service connection... an undocumented gap this build
closes, naming the field to match infra-fleet-service's own
`ConnectionID`/`connectionId` convention"* — i.e. the field exists as a
**placeholder naming convention**, with the caller expected to already know
a resolved connection ID. No parsing/resolution code exists anywhere
(`grep -rn "fleet:tag\|ServerResolver\|resolveServer"` across
`workflow-service`/`orchestration-service`/`infra-fleet-service` returns
nothing). No `provider`/`model`/`accountId` field exists on
`AgentStepConfig` either — `workflow-service.md` §7's TDD text names the
intended priority chain (*"explicit `step.config.provider.accountId` pin
(validated active) beats `ai-provider-service`'s priority-chain resolution
(user > project > server)"*) but none of it is implemented.

## Design — `TargetSpec` parsing + `ServerResolver`

```go
// domain/target_spec.go
type TargetKind int
const (
    TargetKindProject TargetKind = iota // "project:<id>"
    TargetKindServer                    // "server:<id>"
    TargetKindFleetTag                  // "fleet:tag:<tag>"
)
type TargetSpec struct { Kind TargetKind; ID, Tag string }
func ParseTargetSpec(raw string) (TargetSpec, error) { /* prefix-match "project:"/"server:"/"fleet:tag:" */ }
```

```go
// usecase/resolve_target.go
type ServerResolver struct {
    project    projectv1.ProjectServiceClient
    infraFleet infrafleetv1.InfraFleetServiceClient
}
func (r *ServerResolver) Resolve(ctx context.Context, spec domain.TargetSpec) (connectionID string, err error) {
    switch spec.Kind {
    case domain.TargetKindProject:
        proj, err := r.project.GetProject(ctx, &projectv1.GetProjectRequest{Id: spec.ID})
        if err != nil { return "", err }
        return proj.GetDevServerId(), nil // F34's project↔dev-server binding, already real per feature-completion-matrix
    case domain.TargetKindServer:
        return spec.ID, nil
    case domain.TargetKindFleetTag:
        return r.infraFleet.PickByTag(ctx, &infrafleetv1.PickByTagRequest{Tag: spec.Tag})
    }
    return "", domain.ErrUnknownTargetKind
}
```

**Verify before implementing:** `infra-fleet-service`'s `infrafleet.proto`
has no `PickByTag` RPC today (confirmed: not in the 30+ RPC list read
directly off the real proto file) — this solution requires adding it there
too, as a small addition to that service, or reusing whatever load-balancing
primitive already backs `ListDevServersForUser`/fleet-tag-group RPCs
(`AssignDevServerGroup`/`ListDevServerGroups` exist — check whether a
"pick one server from a group" usecase already exists under a different
name before adding a new RPC).

## Design — `ProviderResolver`

```go
// usecase/resolve_provider.go
func (r *ProviderResolver) Resolve(ctx context.Context, cfg domain.AgentStepConfig, projectID, triggeredBy string) (ResolvedProvider, error) {
    if cfg.Provider.AccountID != "" {
        return r.validateActive(ctx, cfg.Provider.AccountID)
    }
    return r.aiProvider.ResolveForContext(ctx, &aiproviderv1.ResolveForContextRequest{UserId: triggeredBy, ProjectId: projectID})
}
```

`AgentStepConfig` gains a `Provider *ProviderPin` field (`{AccountID,
Model string}`, both optional) for the explicit-pin case; `ai-provider-service`'s
existing priority-chain RPC (already built for F35 per the
feature-completion-matrix's ✅) is reused as-is — no new resolution logic
duplicated in `workflow-service`.

## Design — wiring into step executors

```go
// agent_step_executor.go — before building agentExecParams (BE-SOL-001)
spec, err := domain.ParseTargetSpec(cfg.ConnectionID) // NOTE: field name unchanged, semantics widen from "literal ID" to "target spec"
connectionID, err := e.serverResolver.Resolve(ctx, spec)
provider, err := e.providerResolver.Resolve(ctx, cfg, execCtx.ProjectID, execCtx.TriggeredBy)
// then: agentExecParams{..., Model: provider.Model, AccountID: provider.AccountID}
```

`shell_step_executor.go`/`notification_step_executor.go` call
`ServerResolver.Resolve` only (no provider concept for those step types).

## Test plan

- `ParseTargetSpec` on all 3 forms + invalid input.
- `ServerResolver.Resolve("project:<id>")` for a bound vs. unbound project.
- `ProviderResolver.Resolve` priority: explicit pin (active) wins; falls
  back to `ai-provider-service` chain when unset; explicit pin that fails
  validation (inactive account) errors rather than silently falling back.
- End-to-end: 2-step workflow targeting 2 different dev servers dispatches
  to the right one each.

## Not in scope (per the CR)

- `PickByTag`'s load-balancing algorithm — interface only, algorithm choice
  deferred to whoever implements it on `infra-fleet-service`'s side.
- UI for entering `project:`/`server:`/`fleet:tag:` syntax —
  [FE-SOL-001](../../../../../frontend/crs/v4/workflow/solutions/FE-SOL-001-frontend-builder-library-pause-resume.md).

## References

- [CR-WF-002](../../../../../../docs/crs/v4/workflow/CR-WF-002-server-and-provider-resolution.md)
- `specs/backend-go/tdd/services/workflow-service.md` §7
- `backend-go/services/workflow-service/internal/domain/step.go:52-87`
