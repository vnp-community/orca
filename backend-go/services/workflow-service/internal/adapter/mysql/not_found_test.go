//go:build integration

// TASK-E2E-017: a missing row must come back as the domain not-found sentinel, never as a
// generic error (e.g. a raw FK violation) the usecase would map to Internal.
package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

func TestRepository_NotFoundContract(t *testing.T) {
	repo, approvals := setupApprovalStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, id := range []string{uuid.NewString(), "abc"} {
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

	exec := domain.WorkflowExecution{ID: uuid.NewString(), TenantID: tenantID, Status: domain.StatusRunning}
	assertSentinel(t, "UpdateExecution", repo.UpdateExecution(ctx, exec, nil), domain.ErrExecutionNotFound)

	se, err := domain.NewStepExecution(uuid.NewString(), uuid.NewString(), "a", uuid.NewString(), 0)
	if err != nil {
		t.Fatalf("building step execution: %v", err)
	}
	assertSentinel(t, "UpdateStepExecution", repo.UpdateStepExecution(ctx, se, domain.OutboxEvent{}), domain.ErrStepExecutionNotFound)

	err = approvals.WithTx(ctx, func(tx usecase.ApprovalRepositoryTx) error {
		return tx.Update(ctx, domain.Approval{ID: uuid.NewString()})
	})
	assertSentinel(t, "approval Update", err, domain.ErrApprovalNotFound)
}

func assertSentinel(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: want errors.Is(err, %v), got %v", what, want, err)
	}
}
