# TASK-189: Implement `UpdateTemplate` usecase, versioned Postgres write, cycle re-validation, gRPC wiring

**From Solution:** SOL-030
**Priority:** P1
**Service:** `workflow-service`
**File:** `internal/domain/template.go`, `internal/usecase/ports.go`, `internal/usecase/update_template.go` (new), `internal/adapter/postgres/repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-188
**Status:** `[ ]` TODO

---

## Context

`ErrTemplateSelfParent`'s doc comment (`internal/domain/template.go:38-46`)
currently reasons that multi-hop parent cycles can't arise "since there is
no `UpdateTemplate` RPC to rewire an existing template's parent after the
fact." This task removes that invariant's premise: `UpdateTemplate` must
re-validate the new parent's ancestor chain for a cycle back to `id`, using
the already-existing `TemplateRepository.ResolveChain` (the real method
name — not `GetChain`, which SOL-030's own sketch used before checking the
actual port).

## Changes to make

### Step 1 — `internal/domain/template.go`: add `ErrTemplateVersionConflict`, update `ErrTemplateSelfParent`'s doc comment

Add a new sentinel alongside the existing ones:

```go
	// ErrTemplateVersionConflict is the sentinel adapter/postgres returns
	// (wrapped) when UpdateTemplate's conditional UPDATE affects zero rows
	// because templates.version has moved since the caller read it —
	// usecase maps this to apperrors.KindFailedPrecondition.
	ErrTemplateVersionConflict = errors.New("domain: template was modified by another request")
```

Update `ErrTemplateSelfParent`'s doc comment — find:

```go
	// ErrTemplateSelfParent guards against a template naming itself as its
	// own parent, the smallest possible inheritance cycle — see
	// workflow-service.md §4: "Constructor rejects a template naming
	// itself as its own parent, directly." Multi-hop cycles can't arise
	// through this service's RPC surface (a parent must already exist,
	// with its own id assigned, before a child can reference it — there is
	// no UpdateTemplate RPC to rewire an existing template's parent after
	// the fact), so ResolveChain's depth cap (5) is the only additional
	// safety net needed, not full graph cycle detection.
	ErrTemplateSelfParent = errors.New("domain: a template cannot be its own parent")
```

Replace with:

```go
	// ErrTemplateSelfParent guards against a template naming itself as its
	// own parent, the smallest possible inheritance cycle — see
	// workflow-service.md §4: "Constructor rejects a template naming
	// itself as its own parent, directly." UpdateTemplate (added after this
	// comment was first written) CAN rewire an existing template's parent,
	// so a multi-hop cycle is reachable through this service's RPC surface
	// now — see usecase.UpdateTemplate's cycle re-validation, which walks
	// the new parent's ResolveChain and rejects if id appears in it. This
	// constructor-level check still catches the direct self-parent case on
	// every construction path (Create AND Update), independent of that
	// re-validation.
	ErrTemplateSelfParent = errors.New("domain: a template cannot be its own parent")
```

### Step 2 — `internal/usecase/ports.go`: add `Update` to `TemplateRepository`

Find:

```go
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error
	GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error)
	ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error)
	ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error)
}
```

Replace with:

```go
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error
	GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error)
	ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error)
	ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error)
	// Update performs the version-bump-on-write conditional UPDATE.
	// Returns domain.ErrTemplateVersionConflict (wrapped) when
	// expectedVersion doesn't match the current row's version.
	Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error)
}
```

Also add `Version int32` to `domain.WorkflowTemplate` in
`internal/domain/template.go`:

```go
type WorkflowTemplate struct {
	ID       string
	TenantID string
	Name     string
	DAGJSON  string
	Scope    Scope
	ParentTemplateID string
	Version  int32 // bumped by UpdateTemplate on every write; 1 at creation
}
```

`NewWorkflowTemplate`'s constructor return needs `Version: 1` added to its
final `return WorkflowTemplate{...}` literal — every freshly constructed
template starts at version 1.

### Step 3 — `internal/adapter/postgres/repository.go`: implement `Update`, extend `GetTemplate`/`CreateTemplate`/`ListTemplates` for `version`

`CreateTemplate`'s INSERT needs no change (the column defaults to 1 in the
schema). `GetTemplate`/`ListTemplates`' `SELECT`s need `version` added to
their column list and `Scan` targets — find `GetTemplate`:

```go
func (r *Repository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, '')
		FROM workflow.templates
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var tmpl domain.WorkflowTemplate
	var scope string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID)
```

Replace with:

```go
func (r *Repository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
		FROM workflow.templates
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var tmpl domain.WorkflowTemplate
	var scope string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version)
```

Apply the analogous `, version` / `&tmpl.Version` addition to
`ListTemplates`' query and scan loop (same file, directly below
`GetTemplate`).

Add `Update` after `GetTemplate`:

```go
// Update performs the version-bump-on-write conditional UPDATE — the
// versioning rule this solution adds (SOL-030), mirroring SOL-001's
// AccessPolicy pattern. pgx.ErrNoRows here is unambiguous: the caller
// (usecase.UpdateTemplate) already confirmed the row exists via GetTemplate
// before calling this, so a zero-row UPDATE can only mean the version
// moved between that read and this write.
func (r *Repository) Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE workflow.templates
		SET name = $1, dag_json = $2::jsonb, scope = $3, parent_template_id = NULLIF($4, ''),
		    version = version + 1, updated_at = now()
		WHERE id = $5 AND tenant_id = $6 AND version = $7
		RETURNING id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
	`, t.Name, t.DAGJSON, string(t.Scope), t.ParentTemplateID, t.ID, t.TenantID, expectedVersion)

	var updated domain.WorkflowTemplate
	var scope string
	err := row.Scan(&updated.ID, &updated.TenantID, &updated.Name, &updated.DAGJSON, &scope, &updated.ParentTemplateID, &updated.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateVersionConflict
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: update template: %w", err)
	}
	updated.Scope = domain.Scope(scope)
	return updated, nil
}
```

Confirm whether `workflow.templates` already has an `updated_at` column
before including that clause — check the migration under
`services/workflow-service/migrations/`; drop `updated_at = now()` from the
`SET` clause if it doesn't exist rather than adding a new column as a side
effect of this task.

### Step 4 — `internal/usecase/update_template.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

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
	if _, err := uc.templates.GetTemplate(ctx, tenantID, in.ID); err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "template does not exist", nil)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_LOOKUP_FAILED", "failed to look up template", err)
	}

	next, err := domain.NewWorkflowTemplate(in.ID, tenantID, in.Name, in.DAGJSON, in.Scope, in.ParentTemplateID)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	// Cycle re-validation — template.go's ErrTemplateSelfParent doc comment
	// used to reason this was unreachable because no UpdateTemplate existed
	// to rewire a parent after creation. That premise no longer holds: walk
	// the NEW parent's own chain and reject if in.ID appears in it (a
	// multi-hop cycle), not just the direct self-parent case
	// NewWorkflowTemplate already checks.
	if in.ParentTemplateID != "" {
		chain, err := uc.templates.ResolveChain(ctx, tenantID, in.ParentTemplateID, maxTemplateChainDepth)
		if err != nil {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_CHAIN_FAILED", "failed to resolve parent chain", err)
		}
		for _, ancestor := range chain {
			if ancestor.ID == in.ID {
				return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_TEMPLATE_CYCLE", "update would create a cyclic parent chain", nil)
			}
		}
	}

	updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionConflict) {
			return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT", "template was modified by another request", err)
		}
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_UPDATE_TEMPLATE_FAILED", "failed to update template", err)
	}
	// No HasActiveExecutions-style guard needed — DefinitionSnapshot
	// freezes at Execute time (workflow-service.md §4), so this update can
	// never retroactively change a running execution's behavior.
	return updated, nil
}
```

`maxTemplateChainDepth` is the existing constant defined in
`internal/usecase/resolve_template.go` (`const maxTemplateChainDepth = 5`)
— reuse it, do not redeclare.

### Step 5 — `internal/adapter/grpc/server.go`: wire `UpdateTemplate`

Add `updateTemplate *usecase.UpdateTemplate` to `Server`'s struct and
`New`'s params (alongside `resolveTemplate`). Add the handler:

```go
func (s *Server) UpdateTemplate(ctx context.Context, req *workflowv1.UpdateTemplateRequest) (*workflowv1.UpdateTemplateResponse, error) {
	updated, err := s.updateTemplate.Execute(ctx, usecase.UpdateTemplateInput{
		ID:               req.GetId(),
		Name:             req.GetName(),
		DAGJSON:          req.GetDagJson(),
		Scope:            domain.Scope(req.GetScope()),
		ParentTemplateID: req.GetParentTemplateId(),
		ExpectedVersion:  req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.UpdateTemplateResponse{Template: toProtoTemplate(updated)}, nil
}
```

Check whether this file already has a `toProtoTemplate` helper (likely
used by `CreateTemplate`'s handler) — reuse it, adding `Version:
tmpl.Version` to its output if it doesn't already map that field. If no
such helper exists, mirror `CreateTemplate`'s inline construction of
`*workflowv1.WorkflowTemplate` instead.

### Step 6 — `cmd/server/main.go`: construct and wire `UpdateTemplate`

```go
updateTemplateUC := usecase.NewUpdateTemplate(templateRepo)
```

Add to the existing `workflowgrpc.New(...)` call, alongside
`resolveTemplateUC`. `templateRepo` is whatever variable name this file's
existing `CreateTemplate`/`ResolveTemplate` construction already uses for
the `*postgres.Repository` implementing `TemplateRepository` — reuse it.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/workflow-service
go build ./... && go vet ./...
```
