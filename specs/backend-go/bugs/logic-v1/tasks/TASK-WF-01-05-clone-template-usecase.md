# TASK-WF-01-05: Add `CloneTemplate` usecase

**From Solution:** SOL-WF-01
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/clone_template.go` (new)
**Depends on:** TASK-WF-01-02, TASK-WF-01-03, TASK-WF-01-04
**Status:** `[ ]` TODO

---

## Context

BUG-WF-01 finds no disconnected-copy creation path at all. Clone must
snapshot the source template's *resolved* (post-inheritance) DAG, not its
raw `dag_json` — which for an Inherit-mode source may be empty or
override-only and meaningless standalone. This is a new, separate usecase
rather than a flag on `CreateTemplateInput`, matching this codebase's
one-usecase-per-RPC granularity.

## Changes to make

Create `backend-go/services/workflow-service/internal/usecase/clone_template.go`:

```go
package usecase

type CloneTemplateInput struct {
    SourceTemplateID  string
    Name, Description string
    Tags              []string
}

type CloneTemplate struct {
    resolve *ResolveTemplate // reused as-is — Clone is a thin usecase on top
    repo    TemplateRepository
}

func NewCloneTemplate(resolve *ResolveTemplate, repo TemplateRepository) *CloneTemplate {
    return &CloneTemplate{resolve: resolve, repo: repo}
}

func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
    resolved, err := uc.resolve.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
    if err != nil {
        return domain.WorkflowTemplate{}, err
    }

    tenantID := tenant.RequireTenantID(ctx)
    ownerID := identity.RequireUserID(ctx) // match whatever identity accessor create_template.go already uses

    tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.Name, resolved.Template.DAGJSON,
        resolved.Template.Scope, "" /* no parent — Clone is disconnected */, ownerID)
    if err != nil {
        return domain.WorkflowTemplate{}, err
    }
    tmpl.Description = in.Description
    tmpl.Tags = in.Tags
    tmpl.ClonedFromTemplateID = in.SourceTemplateID

    if err := uc.repo.CreateTemplate(ctx, tmpl); err != nil {
        return domain.WorkflowTemplate{}, err
    }
    return tmpl, nil
}
```

Wire `NewCloneTemplate(...)` into `cmd/server/main.go`'s usecase
construction and into the gRPC server's `CloneTemplate` handler
(`internal/adapter/grpc/server.go`), mapping `CloneTemplateRequest` to
`CloneTemplateInput` and `domain.WorkflowTemplate` to
`CloneTemplateResponse`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestCloneTemplate
```

Expected: new `usecase/clone_template_test.go` asserts cloning an
Inherit-mode source (empty own `dag_json`, steps come entirely from its
parent) produces a new root template whose `DAGJSON` matches the
source's *resolved* steps, with `ParentTemplateID` empty and
`ClonedFromTemplateID` set to the source id; a subsequent `UpdateTemplate`
on the clone never touches the original.
