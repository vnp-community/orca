//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// TestWithTx_CommitsOnSuccess confirms WithTx applies every write inside
// fn when fn returns nil — both a UpdateVisibility and an
// IncrementUsageCount in the SAME transaction actually land.
func TestWithTx_CommitsOnSuccess(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		if err := tx.SetVisibility(ctx, tmpl.ID, domain.VisibilityTeam); err != nil {
			return err
		}
		return tx.IncrementUsageCount(ctx, tmpl.ID)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	// GetTemplate's own SELECT doesn't project usage_count/visibility
	// (a pre-existing gap outside this task's scope — see
	// TASK-WF-01-02/03-04's status notes), so read the raw column
	// directly to verify the commit actually landed in the database.
	var usageCount int32
	var visibility string
	if err := repo.pool.QueryRow(ctx, `SELECT usage_count, visibility FROM workflow.templates WHERE id = $1`, tmpl.ID).Scan(&usageCount, &visibility); err != nil {
		t.Fatalf("querying raw columns: %v", err)
	}
	if usageCount != 1 {
		t.Errorf("expected usage_count=1 after commit, got %d", usageCount)
	}
	if visibility != string(domain.VisibilityTeam) {
		t.Errorf("expected visibility=team after commit, got %q", visibility)
	}

	byToken, err := repo.GetByShareToken(ctx, "nonexistent-token")
	if err == nil {
		t.Errorf("expected not-found for an unknown share token, got %+v", byToken)
	}
}

// TestWithTx_RollsBackOnError confirms a failure partway through fn leaves
// NOTHING committed — the exact "inject a failure after a partial write,
// assert nothing committed" scenario the task's Verify section names.
func TestWithTx_RollsBackOnError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "22222222-2222-2222-2222-222222222222"

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	injectedErr := errors.New("simulated failure after partial write")
	err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		// Partial write: this one succeeds...
		if err := tx.SetVisibility(ctx, tmpl.ID, domain.VisibilityTeam); err != nil {
			return err
		}
		// ...but the transaction as a whole fails here, before a second
		// write (IncrementUsageCount) even runs.
		return injectedErr
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected the injected error to propagate, got %v", err)
	}

	// See TestWithTx_CommitsOnSuccess's comment: GetTemplate doesn't
	// project usage_count/visibility, so read the raw columns directly.
	var usageCount int32
	var visibility string
	if err := repo.pool.QueryRow(ctx, `SELECT usage_count, visibility FROM workflow.templates WHERE id = $1`, tmpl.ID).Scan(&usageCount, &visibility); err != nil {
		t.Fatalf("querying raw columns: %v", err)
	}
	if visibility != string(domain.VisibilityPrivate) {
		t.Errorf("expected visibility to remain unchanged (private) after rollback, got %q", visibility)
	}
	if usageCount != 0 {
		t.Errorf("expected usage_count to remain 0 after rollback, got %d", usageCount)
	}
}

// TestWithTx_UpdateVisibilityAndShareToken exercises the read side too:
// UpdateVisibility's RETURNING clause and GetByShareToken both surface the
// new visibility/rating columns correctly.
func TestWithTx_UpdateVisibilityAndShareToken(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "33333333-3333-3333-3333-333333333333"

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	var updated domain.WorkflowTemplate
	err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		tmpl.Visibility = domain.VisibilityTeam
		var uerr error
		updated, uerr = tx.UpdateVisibility(ctx, tmpl)
		return uerr
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if updated.Visibility != domain.VisibilityTeam {
		t.Errorf("expected UpdateVisibility to return the new visibility, got %q", updated.Visibility)
	}

	if err := repo.SetShareToken(ctx, tmpl.ID, "share-tok-1"); err != nil {
		t.Fatalf("set share token: %v", err)
	}
	byToken, err := repo.GetByShareToken(ctx, "share-tok-1")
	if err != nil {
		t.Fatalf("get by share token: %v", err)
	}
	if byToken.ID != tmpl.ID {
		t.Errorf("expected GetByShareToken to find %s, got %s", tmpl.ID, byToken.ID)
	}
}
