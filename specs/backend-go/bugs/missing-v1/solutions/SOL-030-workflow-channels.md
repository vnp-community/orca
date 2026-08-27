# SOL-030: Wire `workflow.execute`/`cancel`/`template.create`, add `UpdateTemplate` for `workflow.template.update`

**Resolves:** [BUG-030](../BUG-030-workflow-channels-not-implemented.md)
**Service:** `workflow-service` (1 new RPC) + `api-gateway` (4 new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/workflow/v1/workflow.proto`
- `backend-go/services/workflow-service/internal/domain/template.go`
- `backend-go/services/workflow-service/internal/usecase/update_template.go` (new)
- `backend-go/services/workflow-service/internal/usecase/ports.go`
- `backend-go/services/workflow-service/internal/adapter/postgres/template_repository.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## Three of four methods are wiring-only — the fourth needs real design work

BUG-030 already confirmed `workflow.execute`/`workflow.cancel`/
`workflow.template.create` are backed by real, REST-proxied RPCs
(`Execute`/`CancelExecution`/`CreateTemplate`,
`workflow.proto:13,23,12`, wired at `handleExecuteWorkflow`/
`handleCancelExecution`/`handleCreateTemplate` in
`workflow_routes.go:52-74,145-160,210-221`) — those three get a thin
`wscompat` wrapper around the same `workflowv1.WorkflowServiceClient`
already dialed into `api-gateway`, no new service-level work. Only
`workflow.template.update` has zero backing RPC — `workflow.proto`'s
`WorkflowService` (`workflow.proto:12-42`) has no `UpdateTemplate`, and the
gap is explicitly deliberate per two independent citations:
`services/workflow-service/internal/domain/template.go:45` ("no
`UpdateTemplate` RPC to rewire an existing template's parent after
[creation]") and `services/workflow-service/README.md:233`.

---

## Design — `UpdateTemplate` versioning, against `workflow-service.md`'s own schema

`workflow-service.md` doesn't spell out `UpdateTemplate` in its RPC sketch
(§3 lists only `CreateTemplate`/`GetTemplate`/`DeleteTemplate`/
`ListTemplates`/`ResolveTemplate`/`Execute`/... — no `UpdateTemplate`
either, so this is a genuine scope addition, flagged as such, not a TDD
citation like SOL-028's `ListTeams`/`RemoveTeamMember` were). But the
**data model** the TDD does specify settles the versioning question:
`templates.version INT NOT NULL DEFAULT 1` (`workflow-service.md:154`) is
already in the schema, unused by any RPC today because nothing ever
updates a template in place. This is the identical shape SOL-001 used for
`auth-service`'s `AccessPolicy` (`auth-service.md:150`'s "an UPDATE creates
a new version row, never an in-place mutation... OPA bundle sync and audit
both need a stable history") — apply the same rule here: **`UpdateTemplate`
must bump `version`, and should not mutate a template a running execution
already snapshotted.**

This last point is the one genuinely new design constraint beyond SOL-001's
precedent, and it's already guaranteed by this service's own existing
design: `WorkflowExecution.DefinitionSnapshot` is "resolved, post-inheritance
steps, **frozen at `Execute` time** so a mid-run template edit never changes
a running execution" (`workflow-service.md:117-118`, §4). `UpdateTemplate`
therefore needs no execution-in-progress guard at all — unlike
`project-service`'s `RebindDevServer` (which *does* need one, because
rebinding a live dev server has an active-execution hazard `DefinitionSnapshot`
doesn't have for templates). Editing a template only affects executions
started *after* the edit; nothing needs to check `HasActiveExecutions`
before allowing it.

The other real constraint: `ErrTemplateSelfParent`'s doc comment
(`template.go:38-46`) currently reasons that multi-hop parent cycles can't
arise "since there is no `UpdateTemplate` RPC to rewire an existing
template's parent after the fact" — this solution removes that invariant's
premise, so `UpdateTemplate` must re-validate against cycles, not just the
direct self-parent case `NewWorkflowTemplate`'s constructor already guards.

---

## Design — Proto addition (`workflow.proto`)

```protobuf
// Deliberate scope addition beyond workflow-service.md §3 — see this
// solution's design section for why it's needed and how it's shaped.
rpc UpdateTemplate(UpdateTemplateRequest) returns (UpdateTemplateResponse);

message UpdateTemplateRequest {
  string id = 1;
  // Field-mask-free: every field is always sent (matches CreateTemplateRequest's
  // shape, no PATCH-style partial update in this reduced surface — same
  // simplification project-service.md's UpdateProject accepts for its own
  // non-dev_server_id fields).
  string name = 2;
  string dag_json = 3;
  string scope = 4;
  string parent_template_id = 5; // empty = detach from any parent
  int32 expected_version = 6;    // optimistic concurrency: reject if templates.version has moved
}

message UpdateTemplateResponse {
  WorkflowTemplate template = 1; // includes the bumped version — add `int32 version = 7` to WorkflowTemplate
}
```

`WorkflowTemplate` (`workflow.proto:54-61`) gains `int32 version = 7` —
additive, `buf breaking` passes.

`expected_version` closes the same lost-update race SOL-001's
`UpdateAccessPolicy` design doesn't explicitly call out but should have:
two concurrent editors both reading `version=3` and both writing would
otherwise silently produce two `version=4` rows or a last-write-wins clobber
depending on implementation; a `WHERE version = $expected` conditional
update makes the loser get a typed `TEMPLATE_VERSION_CONFLICT` instead.

---

## Design — `usecase/` layer

```go
// internal/usecase/ports.go — TemplateRepository grows two methods
type TemplateRepository interface {
    Create(ctx context.Context, t domain.WorkflowTemplate) (domain.WorkflowTemplate, error)
    Get(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, bool, error)
    // GetChain backs ResolveTemplate already — reused here for cycle
    // re-validation (see below), not duplicated.
    GetChain(ctx context.Context, tenantID, id string) ([]domain.WorkflowTemplate, error)
    // Update performs the version-bump-on-write per template.go's new
    // versioning rule. Returns ErrTemplateVersionConflict (sentinel, mapped
    // to apperrors.KindFailedPrecondition) when expectedVersion doesn't
    // match the current row.
    Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error)
}
```

```go
// internal/usecase/update_template.go
type UpdateTemplateInput struct {
    ID, Name, DAGJSON, ParentTemplateID string
    Scope                               domain.Scope
    ExpectedVersion                     int32
}

type UpdateTemplate struct {
    templates TemplateRepository
}

func NewUpdateTemplate(templates TemplateRepository) *UpdateTemplate {
    return &UpdateTemplate{templates: templates}
}

func (uc *UpdateTemplate) Execute(ctx context.Context, in UpdateTemplateInput) (domain.WorkflowTemplate, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
    }
    current, found, err := uc.templates.Get(ctx, tenantID, in.ID)
    if err != nil {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_LOOKUP_FAILED", "failed to look up template", err)
    }
    if !found {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "template does not exist", nil)
    }

    next, err := domain.NewWorkflowTemplate(in.ID, tenantID, in.Name, in.DAGJSON, in.Scope, in.ParentTemplateID)
    if err != nil {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
    }

    // Cycle re-validation — template.go's ErrTemplateSelfParent doc comment
    // (template.go:38-46) reasoned this was unreachable because no
    // UpdateTemplate existed to rewire a parent after creation. That premise
    // no longer holds once this RPC ships: walk the NEW parent's own chain
    // and reject if in.ID appears in it (a multi-hop cycle), not just the
    // direct self-parent case NewWorkflowTemplate already checks.
    if in.ParentTemplateID != "" {
        chain, err := uc.templates.GetChain(ctx, tenantID, in.ParentTemplateID)
        if err != nil {
            return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_CHAIN_FAILED", "failed to resolve parent chain", err)
        }
        for _, ancestor := range chain {
            if ancestor.ID == in.ID {
                return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_CYCLE", "update would create a cyclic parent chain", nil)
            }
        }
    }
    _ = current // current fetched to confirm tenant-scoped existence before the conditional update

    updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion)
    if err != nil {
        if errors.Is(err, domain.ErrTemplateVersionConflict) {
            return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT", "template was modified by another request", err)
        }
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_UPDATE_TEMPLATE_FAILED", "failed to update template", err)
    }
    // No HasActiveExecutions-style guard needed — DefinitionSnapshot freezes
    // at Execute time (workflow-service.md §4), so this update can never
    // retroactively change a running execution's behavior.
    return updated, nil
}
```

`adapter/postgres/template_repository.go`'s `Update` implementation is a
single conditional statement:

```go
func (r *TemplateRepository) Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error) {
    row := r.pool.QueryRow(ctx, `
        UPDATE workflow.templates
        SET name = $1, dag_json = $2, scope = $3, parent_template_id = NULLIF($4, ''),
            version = version + 1, updated_at = now()
        WHERE id = $5 AND tenant_id = $6 AND version = $7
        RETURNING id, tenant_id, name, dag_json, scope, parent_template_id, version
    `, t.Name, t.DAGJSON, t.Scope, t.ParentTemplateID, t.ID, t.TenantID, expectedVersion)
    // Scan; pgx.ErrNoRows here is ambiguous between "not found" and "version
    // conflict" — the usecase already confirmed existence via Get above, so
    // ErrNoRows at this point is unambiguously ErrTemplateVersionConflict.
}
```

---

## Design — `wscompat` wiring (`api-gateway`)

```go
func registerWorkflowChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
    r.Register("workflow.execute", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type executeArgs struct {
            TemplateID  string `json:"templateId"`
            ProjectID   string `json:"projectId"`
            RootTraceID string `json:"rootTraceId"`
            RequestID   string `json:"requestId"`
        }
        in, err := decodeArg[executeArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.Execute(ctx, &workflowv1.ExecuteRequest{
            TemplateId: in.TemplateID, ProjectId: in.ProjectID,
            RootTraceId: in.RootTraceID, RequestId: in.RequestID,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetExecution(), nil
    })

    r.Register("workflow.cancel", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type cancelArgs struct {
            ExecutionID string `json:"executionId"`
        }
        in, err := decodeArg[cancelArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.CancelExecution(ctx, &workflowv1.CancelExecutionRequest{Id: in.ExecutionID})
        if err != nil {
            return nil, err
        }
        return resp.GetExecution(), nil
    })

    r.Register("workflow.template.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            Name             string `json:"name"`
            DAGJSON          string `json:"dagJson"`
            Scope            string `json:"scope"`
            ParentTemplateID string `json:"parentTemplateId"`
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.CreateTemplate(ctx, &workflowv1.CreateTemplateRequest{
            TenantId: id.TenantID, Name: in.Name, DagJson: in.DAGJSON,
            Scope: in.Scope, ParentTemplateId: in.ParentTemplateID,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetTemplate(), nil
    })

    r.Register("workflow.template.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type updateArgs struct {
            ID               string `json:"id"`
            Name             string `json:"name"`
            DAGJSON          string `json:"dagJson"`
            Scope            string `json:"scope"`
            ParentTemplateID string `json:"parentTemplateId"`
            ExpectedVersion  int32  `json:"expectedVersion"`
        }
        in, err := decodeArg[updateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.UpdateTemplate(ctx, &workflowv1.UpdateTemplateRequest{
            Id: in.ID, Name: in.Name, DagJson: in.DAGJSON, Scope: in.Scope,
            ParentTemplateId: in.ParentTemplateID, ExpectedVersion: in.ExpectedVersion,
        })
        if err != nil {
            return nil, err
        }
        return resp.GetTemplate(), nil
    })
}
```

`RegisterRealChannels` (`channels.go:64-82`) grows
`registerWorkflowChannels(r, workflowClient)`; `main.go` dials
`workflow-service`'s gRPC client the same way it already dials
`taskv1`/`gitgatewayv1`/`automationv1` (this client is not currently passed
into `RegisterRealChannels`, per BUG-030's own "falls through to
`notImplementedHandler`" finding — flag as an additional composition-root
wiring change).

All four handlers use `AttachIdentity` since `workflow-service`'s
`Execute`/`CancelExecution`/`CreateTemplate` bind `tenant_id` from context
(`CreateTemplateRequest.tenant_id` is set explicitly in the REST handler
via `identity.TenantID`, `workflow_routes.go:64`, but the gRPC metadata
path is the one `AttachIdentity` establishes for every other call in this
namespace including `CancelExecution`/`Execute`, which carry no
`tenant_id` field at all — `workflow.proto:75-80,144-146`).

---

## Test plan

- `services/workflow-service/internal/usecase/update_template_test.go` —
  happy path bumps `version`; stale `expected_version` returns
  `ErrTemplateVersionConflict` → `WORKFLOW_TEMPLATE_VERSION_CONFLICT`;
  setting `parent_template_id` to an existing descendant of `id` returns
  `WORKFLOW_TEMPLATE_CYCLE`; updating a template that has a completed
  execution referencing it via `DefinitionSnapshot` succeeds and does not
  touch `executions.definition_snapshot` (regression guard on the
  frozen-snapshot invariant).
- `services/workflow-service/internal/adapter/postgres/template_repository_test.go`
  (testcontainers-go) — `Update` with the correct `expectedVersion`
  succeeds and increments the row's `version`; with a stale version affects
  zero rows.
- `services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go`
  — one test per `workflow.*` channel against a fake
  `WorkflowServiceClient`; `workflow.template.update`'s version-conflict
  path surfaces the gRPC `FAILED_PRECONDITION` status as the handler's
  returned error, not swallowed.
- Contract test: `workflow.execute`/`.cancel`/`.template.create` over
  `wscompat` and the equivalent `/v1/workflows/*` REST calls both resolve
  to the same RPC — assert identical response shape (mirrors SOL-001's
  `/admin/api/audit` vs. `/v1/auth/audit-log` regression guard).

## References

- `specs/backend-go/tdd/services/workflow-service.md:53-71` (§3) — current target RPC sketch (no `UpdateTemplate` — scope addition, flagged)
- `specs/backend-go/tdd/services/workflow-service.md:109-120` (§4) — `WorkflowExecution.DefinitionSnapshot` frozen-at-`Execute`-time invariant, why no active-execution guard is needed
- `specs/backend-go/tdd/services/workflow-service.md:149-163` (§5) — `templates.version` column already in the schema, unused until now
- `backend-go/proto/orca/workflow/v1/workflow.proto:11-42,54-73` — current `WorkflowService` surface, `WorkflowTemplate`/`CreateTemplateRequest` messages
- `backend-go/services/workflow-service/internal/domain/template.go:29-46` — `ErrTemplateSelfParent`'s doc comment, the premise this solution's cycle re-check removes
- `backend-go/services/workflow-service/README.md:233` — documented `UpdateTemplate` gap
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:28-38,52-74,145-221` — existing REST→gRPC proxy, all handler line refs above
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:64-82,93-175` — `RegisterRealChannels`, handler pattern to mirror
- `specs/backend-go/bugs/missing-v1/solutions/SOL-001-admin-console-rest-routes.md` — `UpdateAccessPolicy` versioning precedent this solution follows and extends (adds `expected_version`)
- [BUG-030](../BUG-030-workflow-channels-not-implemented.md) — full findings this solution builds on
