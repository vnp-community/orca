//go:build integration

// No internal/adapter/postgres/approval_repository_test.go exists to
// mirror (the Postgres adapter's ApprovalStore has no dedicated adapter-
// level test file — it's only exercised indirectly through usecase-level
// fakes) — these tests are new, added because CreateTx's translation from
// Postgres's partial unique index
// (idx_workflow_approvals_one_pending_per_template ... WHERE status =
// 'pending') to a MySQL generated-column unique index (see
// migrations/mysql/0009_template_visibility_sharing.up.sql's comment) is
// a genuinely nontrivial dialect translation that deserves its own
// verification, not just a build-passes assumption.
package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

func setupApprovalStore(t *testing.T) (*Repository, *ApprovalStore) {
	t.Helper()
	repo := setupRepository(t)
	return repo, NewApprovalStore(repo.db)
}

func TestApprovalStore_CreateGetUpdateRoundTrip(t *testing.T) {
	repo, store := setupApprovalStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "publish-me", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	approval, err := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, "requester-1")
	if err != nil {
		t.Fatalf("building approval: %v", err)
	}

	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.CreateTx(ctx, approval)
	}); err != nil {
		t.Fatalf("create approval: %v", err)
	}

	var got domain.Approval
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		var gerr error
		got, gerr = tx.Get(ctx, approval.ID)
		return gerr
	}); err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if got.Status != domain.ApprovalPending || got.TemplateID != tmpl.ID {
		t.Fatalf("unexpected approval read back: %+v", got)
	}

	if err := got.Approve("admin-1", time.Now().UTC()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.Update(ctx, got)
	}); err != nil {
		t.Fatalf("update approval: %v", err)
	}

	var reread domain.Approval
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		var gerr error
		reread, gerr = tx.Get(ctx, approval.ID)
		return gerr
	}); err != nil {
		t.Fatalf("re-get approval: %v", err)
	}
	if reread.Status != domain.ApprovalApproved || reread.ResolvedBy != "admin-1" || reread.ResolvedAt == nil {
		t.Fatalf("expected approved status/resolvedBy/resolvedAt to persist, got %+v", reread)
	}
}

// TestApprovalStore_CreateTx_SecondPendingForSameTemplateConflicts proves
// the generated-column unique index
// (idx_workflow_approvals_one_pending_per_template) this dialect
// substitutes for Postgres's partial unique index actually rejects a
// second concurrent pending approval for the same template — see this
// file's package doc comment and CreateTx's doc comment.
func TestApprovalStore_CreateTx_SecondPendingForSameTemplateConflicts(t *testing.T) {
	repo, store := setupApprovalStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "publish-me", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	first, _ := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, "requester-1")
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.CreateTx(ctx, first)
	}); err != nil {
		t.Fatalf("create first approval: %v", err)
	}

	second, _ := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, "requester-2")
	err = store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.CreateTx(ctx, second)
	})
	if !errors.Is(err, domain.ErrApprovalAlreadyPending) {
		t.Fatalf("expected domain.ErrApprovalAlreadyPending for a second pending approval on the same template, got %v", err)
	}

	// A NEW pending approval for the same template must be accepted once
	// the first one is resolved — the generated column
	// (`pending_template_id`) must go back to NULL once status != pending,
	// freeing the unique slot (proves this isn't accidentally a permanent
	// one-approval-ever-per-template constraint).
	if err := first.Approve("admin-1", time.Now().UTC()); err != nil {
		t.Fatalf("approve first: %v", err)
	}
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.Update(ctx, first)
	}); err != nil {
		t.Fatalf("resolve first approval: %v", err)
	}

	third, _ := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, "requester-3")
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.CreateTx(ctx, third)
	}); err != nil {
		t.Fatalf("expected a new pending approval to be accepted once the prior one resolved, got %v", err)
	}
}

func TestApprovalStore_Get_NotFound(t *testing.T) {
	_, store := setupApprovalStore(t)
	ctx := context.Background()

	err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		_, gerr := tx.Get(ctx, uuid.NewString())
		return gerr
	})
	if !errors.Is(err, domain.ErrApprovalNotFound) {
		t.Fatalf("expected domain.ErrApprovalNotFound, got %v", err)
	}
}

// TestApprovalStore_ListPending_DoesNotLeakAcrossTenants — see this
// package's repository.go doc comment (TASK-BE-DB-003's pattern):
// approvals carries its own tenant_id and had an RLS policy in the
// Postgres migration, which this dialect has no equivalent for.
func TestApprovalStore_ListPending_DoesNotLeakAcrossTenants(t *testing.T) {
	repo, store := setupApprovalStore(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.NewString(), uuid.NewString()

	tmplA, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantA, "a", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmplA)
	tmplB, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantB, "b", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	_ = repo.CreateTemplate(ctx, tmplB)

	approvalA, _ := domain.NewApproval(uuid.NewString(), tenantA, tmplA.ID, "requester-a")
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error { return tx.CreateTx(ctx, approvalA) }); err != nil {
		t.Fatalf("create approval a: %v", err)
	}
	approvalB, _ := domain.NewApproval(uuid.NewString(), tenantB, tmplB.ID, "requester-b")
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error { return tx.CreateTx(ctx, approvalB) }); err != nil {
		t.Fatalf("create approval b: %v", err)
	}

	got, _, err := store.ListPending(ctx, tenantA, "", 50)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(got) != 1 || got[0].ID != approvalA.ID {
		t.Fatalf("expected only tenant A's pending approval, got %+v", got)
	}
}

// TestApprovalStore_Templates_SharesTransactionWithApproval exercises
// ApprovalRepositoryTx.Templates()' bridge — a TemplateRepositoryTx write
// through the SAME transaction as an approval write, the mechanism
// usecase.ResolveApproval relies on to apply VisibilityCompany atomically
// with the approval's resolution.
func TestApprovalStore_Templates_SharesTransactionWithApproval(t *testing.T) {
	repo, store := setupApprovalStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, _ := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	approval, _ := domain.NewApproval(uuid.NewString(), tenantID, tmpl.ID, "requester-1")
	if err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error { return tx.CreateTx(ctx, approval) }); err != nil {
		t.Fatalf("create approval: %v", err)
	}

	err := store.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		a, gerr := tx.Get(ctx, approval.ID)
		if gerr != nil {
			return gerr
		}
		if aerr := a.Approve("admin-1", time.Now().UTC()); aerr != nil {
			return aerr
		}
		if uerr := tx.Update(ctx, a); uerr != nil {
			return uerr
		}
		return tx.Templates().SetVisibility(ctx, tmpl.ID, domain.VisibilityCompany)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	var visibility string
	if err := repo.db.QueryRowContext(ctx, `SELECT visibility FROM templates WHERE id = ?`, tmpl.ID).Scan(&visibility); err != nil {
		t.Fatalf("querying raw visibility: %v", err)
	}
	if visibility != string(domain.VisibilityCompany) {
		t.Errorf("expected visibility=company after the shared-transaction write, got %q", visibility)
	}
}
