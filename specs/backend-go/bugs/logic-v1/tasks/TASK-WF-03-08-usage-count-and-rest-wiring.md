# TASK-WF-03-08: Wire usage-count increment into `Execute` + REST/public-preview routes

**From Solution:** SOL-WF-03
**Priority:** P2
**Service:** `workflow-service` + `api-gateway`
**File:** `backend-go/services/workflow-service/internal/usecase/execute.go`
**Depends on:** TASK-WF-03-04, TASK-WF-03-05, TASK-WF-03-06, TASK-WF-03-07
**Status:** `[ ]` TODO

---

## Context

`Execute` doesn't increment `templates.usage_count` today, and none of
this solution's new RPCs are exposed over REST. Usage-count increment is
a same-transaction local write (both rows live in `workflow-service`'s
own database) — not a cross-service outbox/saga case.

## Changes to make

In `internal/usecase/execute.go`, wrap the existing `CreateExecution`
call (`execute.go:114`) in the same transaction as the usage-count
increment:

```go
func (uc *Execute) Execute(ctx context.Context, in ExecuteInput) (domain.WorkflowExecution, error) {
    // ... existing template fetch / DAG validate / wave build unchanged ...
    err := uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
        if err := tx.CreateExecution(ctx, exec); err != nil {
            return err
        }
        return tx.IncrementUsageCount(ctx, tmpl.ID)
    })
    if err != nil {
        return domain.WorkflowExecution{}, err
    }
    // ... unchanged async dispatch ...
}
```

Add to `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go`:

```go
sub.Post("/templates/{id}/publish", handlePublishTemplate(client))
sub.Get("/templates/approvals", handleListPendingApprovals(client))       // admin-only, enforced server-side via OPA
sub.Post("/templates/approvals/{id}/resolve", handleResolveApproval(client))
sub.Post("/templates/{id}/share-link", handleGenerateShareLink(client))
sub.Post("/templates/{id}/rate", handleRateTemplate(client))
```

Mount the two public, non-tenant-authenticated routes outside the
authenticated group (matching `api-gateway`'s existing pattern for a
genuinely public endpoint, e.g. health checks — this is
`workflow-service`'s first tenant-context-free business endpoint):

```go
// mounted at the router root, not under the JWT-required middleware chain
r.Get("/v1/workflows/shared/{token}", handlePreviewSharedTemplate(client))
// this route's PATH is public, but the handler itself calls
// identityFromContext and 401s if absent — the importer's own identity
// is required, only Preview is truly anonymous.
r.Post("/v1/workflows/shared/{token}/import", handleImportSharedTemplate(client))
```

Extend the existing `handleListTemplates` (`workflow_routes.go:77-104`)
to pass through `query`/`tags`/`sort` query params.

Note: `wscompat`'s `workflow.*` namespace registration (tracked by
BUG-030) should include these new RPCs when that gap is picked up — not
wired here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/... ./services/api-gateway/...
go test ./services/workflow-service/internal/usecase/... -run TestExecute
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestWorkflowRoutes
```

Expected: `Execute` increments `usage_count` exactly once per call,
atomically with `CreateExecution` (inject a failure after the increment
but before commit in a fake tx, assert neither side effect landed). The
two public routes are reachable with NO `Authorization` header and never
401 a Preview request; the Import route DOES 401 without one.
