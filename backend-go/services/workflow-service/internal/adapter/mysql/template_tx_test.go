//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// TestWithTx_CommitsOnSuccess mirrors
// internal/adapter/postgres/template_tx_test.go's identical test.
func TestWithTx_CommitsOnSuccess(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

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

	// GetTemplate's own SELECT doesn't project usage_count/visibility (a
	// pre-existing gap mirrored from the Postgres adapter), so read the raw
	// columns directly to verify the commit actually landed.
	var usageCount int32
	var visibility string
	if err := repo.db.QueryRowContext(ctx, `SELECT usage_count, visibility FROM templates WHERE id = ?`, tmpl.ID).Scan(&usageCount, &visibility); err != nil {
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

// TestWithTx_RollsBackOnError mirrors
// internal/adapter/postgres/template_tx_test.go's identical test.
func TestWithTx_RollsBackOnError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	injectedErr := errors.New("simulated failure after partial write")
	err = repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		if err := tx.SetVisibility(ctx, tmpl.ID, domain.VisibilityTeam); err != nil {
			return err
		}
		return injectedErr
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected the injected error to propagate, got %v", err)
	}

	var usageCount int32
	var visibility string
	if err := repo.db.QueryRowContext(ctx, `SELECT usage_count, visibility FROM templates WHERE id = ?`, tmpl.ID).Scan(&usageCount, &visibility); err != nil {
		t.Fatalf("querying raw columns: %v", err)
	}
	if visibility != string(domain.VisibilityPrivate) {
		t.Errorf("expected visibility to remain unchanged (private) after rollback, got %q", visibility)
	}
	if usageCount != 0 {
		t.Errorf("expected usage_count to remain 0 after rollback, got %d", usageCount)
	}
}

// TestWithTx_UpdateVisibilityAndShareToken mirrors
// internal/adapter/postgres/template_tx_test.go's identical test.
func TestWithTx_UpdateVisibilityAndShareToken(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

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

// TestWithTx_UpdateVisibility_NotFound proves the SELECT ... FOR UPDATE
// existence check this adapter uses in place of the Postgres adapter's
// RETURNING-based RowsAffected()==0 check actually surfaces
// ErrTemplateNotFound for a template that was never created.
func TestWithTx_UpdateVisibility_NotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ghost := domain.WorkflowTemplate{ID: uuid.NewString(), TenantID: uuid.NewString(), Visibility: domain.VisibilityTeam}
	err := repo.WithTx(ctx, func(tx usecase.TemplateRepositoryTx) error {
		_, uerr := tx.UpdateVisibility(ctx, ghost)
		return uerr
	})
	if !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Fatalf("expected domain.ErrTemplateNotFound, got %v", err)
	}
}
