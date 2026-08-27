# TASK-WF-03-04: Add `WithTx`, `ApprovalRepository`, and `OPAChecker` ports

**From Solution:** SOL-WF-03
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/ports.go`
**Depends on:** TASK-WF-03-02
**Status:** `[ ]` TODO

---

## Context

Every existing `workflow-service` usecase is single-statement today —
this is the first multi-statement-atomic requirement in the service
(a template's `visibility` transition and its approval row must commit
together). This task adds the transaction-boundary port shape and the
new `ApprovalRepository`/`OPAChecker` ports that TASK-WF-03-05 through
TASK-WF-03-08 build on. It is new plumbing to `adapter/postgres`, not
just new queries — flag that explicitly to whoever implements it.

## Changes to make

In `internal/usecase/ports.go`, extend `TemplateRepository`:

```go
type TemplateRepositoryTx interface {
    UpdateVisibility(ctx context.Context, tmpl domain.WorkflowTemplate) (domain.WorkflowTemplate, error)
    SetVisibility(ctx context.Context, templateID string, v domain.Visibility) error
    CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error
    IncrementUsageCount(ctx context.Context, templateID string) error
}

type TemplateRepository interface {
    // ... existing methods unchanged ...
    WithTx(ctx context.Context, fn func(tx TemplateRepositoryTx) error) error
    GetByShareToken(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error)
    SetShareToken(ctx context.Context, templateID, token string) error
}

type ApprovalRepositoryTx interface {
    Get(ctx context.Context, approvalID string) (domain.Approval, error)
    Update(ctx context.Context, approval domain.Approval) error
    Templates() TemplateRepositoryTx
    CreateTx(ctx context.Context, approval domain.Approval) error
}

type ApprovalRepository interface {
    WithTx(ctx context.Context, fn func(tx ApprovalRepositoryTx) error) error
    ListPending(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Approval, string, error)
}

// OPAChecker mirrors orchestration-service's in-process OPA policy check
// for ResolveDecisionGate — "requester is a lead"/"approver is an admin"
// are auth-service/tenant-service facts, not something workflow-service
// determines from its own tables.
type OPAChecker interface {
    IsAdmin(ctx context.Context, userID string) bool
}
```

Implement the `WithTx` boundary helper in
`backend-go/services/workflow-service/internal/adapter/postgres/` (a
`pool.WithTransaction`-equivalent that begins a Postgres transaction,
passes a tx-scoped repository implementation into `fn`, commits on
success and rolls back on error) and a new
`internal/adapter/postgres/approval_repository.go` implementing
`ApprovalRepository`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/postgres/... -run TestWithTx
```

Expected: `WithTx` rolls back every write inside `fn` when `fn` returns
an error (inject a failure after a partial write, assert nothing
committed); commits everything when `fn` returns nil.
