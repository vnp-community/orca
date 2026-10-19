//go:build integration

// TASK-E2E-017: a missing (or malformed-uuid) row must come back as the domain
// not-found sentinel, never as a generic error the usecase would map to Internal.
package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

func TestRepository_NotFoundContract(t *testing.T) {
	repo := setupRepository(t)
	approvals := NewApprovalStore(repo.pool)
	ctx := context.Background()
	tenantID := uuid.NewString()
	missing := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "t", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	for _, id := range []string{missing, "abc"} {
		_, err := repo.GetTemplate(ctx, tenantID, id)
		assertSentinel(t, "GetTemplate "+id, err, domain.ErrTemplateNotFound)
		_, err = repo.GetExecution(ctx, tenantID, id)
		assertSentinel(t, "GetExecution "+id, err, domain.ErrExecutionNotFound)
		_, err = repo.ResolveChain(ctx, tenantID, id, 5)
		assertSentinel(t, "ResolveChain "+id, err, domain.ErrTemplateNotFound)
		assertSentinel(t, "SetShareToken "+id, repo.SetShareToken(ctx, id, "tok"), domain.ErrTemplateNotFound)

		err = approvals.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
			_, gerr := tx.Get(ctx, id)
			return gerr
		})
		assertSentinel(t, "approval Get "+id, err, domain.ErrApprovalNotFound)

		err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
			_, rerr := tx.UpsertRating(ctx, id, uuid.NewString(), 4)
			return rerr
		})
		assertSentinel(t, "UpsertRating "+id, err, domain.ErrTemplateNotFound)
		err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error { return tx.SetVisibility(ctx, id, domain.VisibilityPublic) })
		assertSentinel(t, "SetVisibility "+id, err, domain.ErrTemplateNotFound)
		err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error { return tx.IncrementUsageCount(ctx, id) })
		assertSentinel(t, "IncrementUsageCount "+id, err, domain.ErrTemplateNotFound)
	}

	assertSentinel(t, "GetByShareToken", func() error { _, e := repo.GetByShareToken(ctx, "nope"); return e }(), domain.ErrTemplateNotFound)

	exec := domain.WorkflowExecution{ID: missing, TenantID: tenantID, Status: domain.StatusRunning}
	assertSentinel(t, "UpdateExecution", repo.UpdateExecution(ctx, exec, nil), domain.ErrExecutionNotFound)

	se, err := domain.NewStepExecution(uuid.NewString(), missing, "a", uuid.NewString(), 0)
	if err != nil {
		t.Fatalf("building step execution: %v", err)
	}
	assertSentinel(t, "UpdateStepExecution", repo.UpdateStepExecution(ctx, se, domain.OutboxEvent{}), domain.ErrStepExecutionNotFound)

	err = approvals.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.Update(ctx, domain.Approval{ID: missing})
	})
	assertSentinel(t, "approval Update", err, domain.ErrApprovalNotFound)

	// A project id that cannot be a uuid has no executions: empty, not an error.
	has, err := repo.HasActiveExecutions(ctx, tenantID, "abc")
	if err != nil || has {
		t.Fatalf("HasActiveExecutions(malformed project) = %v, %v; want false, nil", has, err)
	}
	list, _, err := repo.ListExecutions(ctx, tenantID, "abc", "", 10)
	if err != nil || len(list) != 0 {
		t.Fatalf("ListExecutions(malformed project) = %v, %v; want empty, nil", list, err)
	}
}

func assertSentinel(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: want errors.Is(err, %v), got %v", what, want, err)
	}
}
