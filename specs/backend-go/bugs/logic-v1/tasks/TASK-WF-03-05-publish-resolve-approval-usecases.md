# TASK-WF-03-05: Implement `PublishTemplate` and `ResolveApproval` usecases

**From Solution:** SOL-WF-03
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/publish_template.go` (new)
**Depends on:** TASK-WF-03-04, TASK-WF-03-03
**Status:** `[x]` DONE — `publish_template.go`/`resolve_approval.go` implement the owner/admin gate, escalate-to-company approval creation, and atomic approve-applies-VisibilityCompany transaction; wired into `cmd/server/main.go` + `adapter/grpc/server.go` (new `internal/adapter/opachecker` client against auth-service) plus a `list_pending_approvals.go` usecase for the proto's `ListPendingApprovals` RPC. `publish_template_test.go`/`resolve_approval_test.go` added (non-owner rejected, owner direct-apply, lead creates pending approval, admin direct-apply, duplicate-pending conflict via `domain.ErrApprovalAlreadyPending`, non-admin resolve rejected, approve applies `VisibilityCompany` atomically, reject leaves visibility unchanged, injected mid-tx failure rolls back both writes, not-found). `go build ./services/workflow-service/...` clean; `go test ./services/workflow-service/internal/usecase/... -run 'TestPublishTemplate|TestResolveApproval'` all pass.

---

## Context

BUG-WF-03's visibility-escalation and lead-requires-admin-approval gate
have no usecase today. Both must be atomic per
`orchestration-service.md` §8's discipline — a torn write could leave a
template silently already-company-visible with no pending approval to
gate it, or an orphaned approval nobody can act on.

## Changes to make

Create `backend-go/services/workflow-service/internal/usecase/publish_template.go`:

```go
package usecase

type PublishTemplateInput struct {
    TemplateID    string
    NewVisibility domain.Visibility
}

type PublishTemplate struct {
    templates TemplateRepository
    approvals ApprovalRepository
    opa       OPAChecker
}

func (uc *PublishTemplate) Execute(ctx context.Context, in PublishTemplateInput) (domain.WorkflowTemplate, error) {
    tenantID := tenant.RequireTenantID(ctx)
    callerUserID := identity.RequireUserID(ctx)

    tmpl, err := uc.templates.GetTemplate(ctx, tenantID, in.TemplateID)
    if err != nil {
        return domain.WorkflowTemplate{}, err
    }
    if tmpl.OwnerID != callerUserID && !uc.opa.IsAdmin(ctx, callerUserID) {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_PUBLISH_NOT_OWNER", "only the template owner or an admin may publish")
    }
    if !tmpl.Visibility.CanEscalateTo(in.NewVisibility) {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_VISIBILITY_TRANSITION", "cannot move directly from %s to %s", tmpl.Visibility, in.NewVisibility)
    }

    if in.NewVisibility == domain.VisibilityCompany && !uc.opa.IsAdmin(ctx, callerUserID) {
        // Lead (non-admin) requesting company scope: create a pending
        // approval, visibility STAYS AT ITS CURRENT TIER until resolved.
        var result domain.WorkflowTemplate
        err := uc.approvals.WithTx(ctx, func(tx ApprovalRepositoryTx) error {
            approval := domain.Approval{ID: uuid.NewString(), TemplateID: tmpl.ID, RequestedBy: callerUserID, Status: domain.ApprovalPending}
            if err := tx.CreateTx(ctx, approval); err != nil {
                return err
            }
            result = tmpl // unchanged visibility — pending, not yet published
            return nil
        })
        return result, err
    }

    // Admin publishing directly, OR any non-company tier escalation, or
    // private unpublish — no approval gate.
    tmpl.Visibility = in.NewVisibility
    var result domain.WorkflowTemplate
    err = uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
        var uerr error
        result, uerr = tx.UpdateVisibility(ctx, tmpl)
        return uerr
    })
    return result, err
}
```

Create `backend-go/services/workflow-service/internal/usecase/resolve_approval.go`:

```go
package usecase

type ResolveApprovalInput struct {
    ApprovalID string
    Decision   domain.ApprovalStatus // Approved | Rejected
}

type ResolveApproval struct {
    approvals ApprovalRepository
    opa       OPAChecker
}

func (uc *ResolveApproval) Execute(ctx context.Context, in ResolveApprovalInput) (domain.Approval, error) {
    callerUserID := identity.RequireUserID(ctx)
    if !uc.opa.IsAdmin(ctx, callerUserID) {
        return domain.Approval{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_APPROVAL_ADMIN_ONLY", "only an admin may resolve an approval")
    }
    var result domain.Approval
    err := uc.approvals.WithTx(ctx, func(tx ApprovalRepositoryTx) error {
        approval, err := tx.Get(ctx, in.ApprovalID)
        if err != nil {
            return err
        }
        now := time.Now()
        approval.Status, approval.ResolvedBy, approval.ResolvedAt = in.Decision, callerUserID, &now
        if err := tx.Update(ctx, approval); err != nil {
            return err
        }
        if in.Decision == domain.ApprovalApproved {
            if err := tx.Templates().SetVisibility(ctx, approval.TemplateID, domain.VisibilityCompany); err != nil {
                return err
            }
        }
        result = approval
        return nil
    })
    return result, err
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestPublishTemplate
go test ./services/workflow-service/internal/usecase/... -run TestResolveApproval
```

Expected: non-owner/non-admin publish rejected; owner escalating
private→team succeeds with no approval row; lead escalating to company
creates a pending `Approval` and leaves `Visibility` unchanged; admin
escalating to company applies immediately with no approval row; a second
company-publish request while one is pending surfaces as a clean, typed
conflict error (from `idx_approvals_one_pending_per_template`), not a raw
constraint-violation leak. `ResolveApproval`: non-admin rejected; approve
applies `VisibilityCompany` atomically with the approval's status flip —
inject a failure between the two writes in a fake tx and assert neither
side effect landed; reject leaves `Visibility` unchanged.
