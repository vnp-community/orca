# TASK-WF-03-06: Implement share-link generate/preview/import usecases

**From Solution:** SOL-WF-03
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/generate_share_link.go` (new)
**Depends on:** TASK-WF-03-04, TASK-WF-03-03, TASK-WF-01-05 (`ImportSharedTemplate` reuses `CloneTemplate`'s "resolved-DAG, no-parent copy" construction pattern from SOL-WF-01)
**Status:** `[ ]` TODO

---

## Context

BUG-WF-03's share-link flow is entirely missing. `PreviewSharedTemplate`
is the one usecase in this service with no tenant/auth context at all —
a deliberate, narrow exception to `05-data-architecture.md`'s
tenant-scoping rule, looked up by an unguessable `share_token` and
returning only a read-only projection.

## Changes to make

Create `backend-go/services/workflow-service/internal/usecase/generate_share_link.go`:

```go
package usecase

// GenerateShareLink is only meaningful once Visibility == public — the
// token IS the access-control boundary for a public template.
func (uc *GenerateShareLink) Execute(ctx context.Context, templateID string) (string, error) {
    tenantID := tenant.RequireTenantID(ctx)
    tmpl, err := uc.templates.GetTemplate(ctx, tenantID, templateID)
    if err != nil {
        return "", err
    }
    if tmpl.Visibility != domain.VisibilityPublic {
        return "", apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_NOT_PUBLIC", "template must be public to generate a share link")
    }
    if tmpl.ShareToken != "" {
        return tmpl.ShareToken, nil // idempotent — re-requesting doesn't rotate the token
    }
    token := generateOpaqueToken() // crypto/rand, base64url
    return token, uc.templates.SetShareToken(ctx, templateID, token)
}
```

Create `backend-go/services/workflow-service/internal/usecase/preview_shared_template.go`:

```go
package usecase

// PreviewSharedTemplate is the ONE read in this service with NO
// tenant/auth context — looked up by share_token (unguessable, not a
// template id an attacker could enumerate), returns a read-only
// projection (name/description/tags/dag_json/rating) — never owner_id,
// never any other tenant's templates, never list/search.
func (uc *PreviewSharedTemplate) Execute(ctx context.Context, shareToken string) (SharedTemplatePreview, error) {
    tmpl, err := uc.templates.GetByShareToken(ctx, shareToken) // no tenantID param — token IS the lookup key
    if err != nil || tmpl.Visibility != domain.VisibilityPublic {
        // Same error for "not found" and "no longer public" — don't leak which.
        return SharedTemplatePreview{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", "share link is invalid or expired")
    }
    return toPreview(tmpl), nil
}
```

Create `backend-go/services/workflow-service/internal/usecase/import_shared_template.go`:

```go
package usecase

// ImportSharedTemplate requires a real, authenticated, tenant-scoped
// caller (unlike Preview). Reuses CloneTemplate's construction shape — a
// personal-scope, no-parent copy — since the only difference from a
// same-tenant Clone is that the SOURCE may belong to a different tenant,
// so the lookup goes through GetByShareToken rather than GetTemplate.
func (uc *ImportSharedTemplate) Execute(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
    source, err := uc.templates.GetByShareToken(ctx, shareToken)
    if err != nil || source.Visibility != domain.VisibilityPublic {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", "share link is invalid or expired")
    }
    // Cross-tenant resolve: ResolveChain called directly with the
    // SOURCE template's own tenant_id (read off the row above), NOT
    // through the tenant-scoped ResolveTemplate usecase — deliberate
    // usecase-boundary decision, not a workaround.
    resolved, err := uc.resolveChain.Resolve(ctx, source.TenantID, source.ID)
    if err != nil {
        return domain.WorkflowTemplate{}, err
    }

    importerTenantID := tenant.RequireTenantID(ctx)
    importerUserID := identity.RequireUserID(ctx)
    tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), importerTenantID, source.Name+" (imported)",
        resolved.DAGJSON, domain.ScopePersonal, "" /* no parent */, importerUserID)
    if err != nil {
        return domain.WorkflowTemplate{}, err
    }
    tmpl.ClonedFromTemplateID = source.ID // provenance across the tenant boundary
    return tmpl, uc.templates.CreateTemplate(ctx, tmpl)
}
```

Note: `ResolveTemplate`'s current signature (`resolve_template.go:56-64`)
pulls `tenantID` from `ctx` — do not weaken it for every same-tenant
caller. Add a lower-level `resolveChain.Resolve(ctx, tenantID, templateID)`
call (bypassing the tenant-scoped `ResolveTemplate` usecase) for this
cross-tenant case only.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestGenerateShareLink
go test ./services/workflow-service/internal/usecase/... -run TestPreviewSharedTemplate
go test ./services/workflow-service/internal/usecase/... -run TestImportSharedTemplate
```

Expected: non-public template rejected by `GenerateShareLink`; repeated
calls on an already-public template return the SAME token; an unknown
token and a valid token whose template has since been unpublished both
return the identical `WORKFLOW_SHARE_LINK_INVALID` error; the preview
response never includes `owner_id`/`tenant_id` (assert via reflection
over the returned struct's field set). Importing a cross-tenant public
template creates a new personal-scope row under the IMPORTER's tenant
with the resolved DAG, empty `ParentTemplateID`, `ClonedFromTemplateID`
set; importing a since-unpublished token fails the same as Preview.
