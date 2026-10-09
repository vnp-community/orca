package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// assertKind checks the usecase error maps to the expected kind/code (TASK-E2E-017 convention).
func assertKind(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != kind || (code != "" && ae.Code != code) {
		t.Errorf("want kind=%v code=%q, got kind=%v code=%q", kind, code, ae.Kind, ae.Code)
	}
}

func TestExecutionLookups_NotFoundVsInfrastructureError(t *testing.T) {
	wrappedNotFound := fmt.Errorf("postgres: execution x: %w", domain.ErrExecutionNotFound)
	infra := errors.New("connection reset")

	run := map[string]func(repo *fakeExecutionRepository) error{
		"get": func(r *fakeExecutionRepository) error {
			_, err := NewGetExecution(r).Execute(withTenantContext(context.Background(), "t1"), GetExecutionInput{ID: "x"})
			return err
		},
		"cancel": func(r *fakeExecutionRepository) error {
			_, err := NewCancelExecution(r).Execute(withTenantContext(context.Background(), "t1"), CancelExecutionInput{ID: "x"})
			return err
		},
		"pause": func(r *fakeExecutionRepository) error {
			_, err := NewPauseExecution(r).Execute(withTenantContext(context.Background(), "t1"), PauseExecutionInput{ID: "x"})
			return err
		},
		"resume": func(r *fakeExecutionRepository) error {
			_, err := NewResumeExecution(r).Execute(withTenantContext(context.Background(), "t1"), ResumeExecutionInput{ID: "x"})
			return err
		},
	}
	for name, fn := range run {
		t.Run(name+"/wrapped sentinel is NotFound", func(t *testing.T) {
			repo := newFakeExecutionRepository()
			repo.getErr = wrappedNotFound
			assertKind(t, fn(repo), apperrors.KindNotFound, "WORKFLOW_EXECUTION_NOT_FOUND")
		})
		t.Run(name+"/other repo error is Internal", func(t *testing.T) {
			repo := newFakeExecutionRepository()
			repo.getErr = infra
			assertKind(t, fn(repo), apperrors.KindInternal, "WORKFLOW_EXECUTION_FETCH_FAILED")
		})
	}
}

func TestUpdateTemplate_NotFoundVsInfrastructureError(t *testing.T) {
	ctx := withTenantContext(context.Background(), "tenant-1")
	in := UpdateTemplateInput{ID: "tmpl-1", Name: "n", DAGJSON: `{"steps":[]}`, Scope: domain.ScopePersonal, ExpectedVersion: 1}

	t.Run("wrapped template sentinel is NotFound", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.getErr = fmt.Errorf("postgres: template: %w", domain.ErrTemplateNotFound)
		_, err := NewUpdateTemplate(repo).Execute(ctx, in)
		assertKind(t, err, apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND")
	})
	t.Run("other lookup error is Internal", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.getErr = errors.New("boom")
		_, err := NewUpdateTemplate(repo).Execute(ctx, in)
		assertKind(t, err, apperrors.KindInternal, "WORKFLOW_TEMPLATE_LOOKUP_FAILED")
	})

	withExisting := func() *fakeTemplateRepository {
		repo := newFakeTemplateRepository()
		existing, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "n", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
		if err != nil {
			t.Fatalf("building template: %v", err)
		}
		repo.templates[existing.ID] = existing
		return repo
	}
	withParent := in
	withParent.ParentTemplateID = "missing-parent"
	t.Run("unknown parent is FailedPrecondition, not Internal", func(t *testing.T) {
		_, err := NewUpdateTemplate(withExisting()).Execute(ctx, withParent)
		assertKind(t, err, apperrors.KindFailedPrecondition, "WORKFLOW_PARENT_TEMPLATE_NOT_FOUND")
	})
	t.Run("parent chain infrastructure error is Internal", func(t *testing.T) {
		repo := withExisting()
		repo.resolveErr = errors.New("boom")
		_, err := NewUpdateTemplate(repo).Execute(ctx, withParent)
		assertKind(t, err, apperrors.KindInternal, "WORKFLOW_TEMPLATE_CHAIN_FAILED")
	})
}

func TestSharedTemplate_InfrastructureErrorIsNotInvalidLink(t *testing.T) {
	ctx := withTenantContext(context.Background(), "tenant-importer")
	infra := errors.New("connection reset")

	t.Run("preview", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.shareTokenErr = infra
		_, err := NewPreviewSharedTemplate(repo).Execute(ctx, "tok")
		assertKind(t, err, apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED")
	})
	t.Run("import", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.shareTokenErr = infra
		_, err := NewImportSharedTemplate(repo, NewResolveTemplate(repo)).Execute(ctx, "tok")
		assertKind(t, err, apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED")
	})
	t.Run("preview wrapped sentinel stays invalid link", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.shareTokenErr = fmt.Errorf("postgres: template by share token: %w", domain.ErrTemplateNotFound)
		_, err := NewPreviewSharedTemplate(repo).Execute(ctx, "tok")
		assertKind(t, err, apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID")
	})
	t.Run("import wrapped sentinel stays invalid link", func(t *testing.T) {
		repo := newFakeTemplateRepository()
		repo.shareTokenErr = fmt.Errorf("postgres: template by share token: %w", domain.ErrTemplateNotFound)
		_, err := NewImportSharedTemplate(repo, NewResolveTemplate(repo)).Execute(ctx, "tok")
		assertKind(t, err, apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID")
	})
}
