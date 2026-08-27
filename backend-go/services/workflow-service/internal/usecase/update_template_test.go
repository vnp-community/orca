package usecase

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestUpdateTemplate_Succeeds_ForwardsExpectedVersionAndReturnsBumpedResult(t *testing.T) {
	repo := newFakeTemplateRepository()
	existing, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	repo.templates[existing.ID] = existing

	uc := NewUpdateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, UpdateTemplateInput{
		ID: "tmpl-1", Name: "deploy-v2", DAGJSON: `{"steps":[]}`, Scope: domain.ScopePersonal, ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("want bumped version 2, got %d", got.Version)
	}
	if got.Name != "deploy-v2" {
		t.Errorf("want name=deploy-v2, got %q", got.Name)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("want exactly 1 Update call, got %d", repo.updateCalls)
	}
	if repo.lastUpdateExpectedVersion != 1 {
		t.Errorf("want expectedVersion=1 forwarded unchanged, got %d", repo.lastUpdateExpectedVersion)
	}
}

func TestUpdateTemplate_StaleExpectedVersion_ReturnsFailedPrecondition(t *testing.T) {
	repo := newFakeTemplateRepository()
	existing, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	repo.templates[existing.ID] = existing
	repo.updateErr = domain.ErrTemplateVersionConflict

	uc := NewUpdateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err = uc.Execute(ctx, UpdateTemplateInput{
		ID: "tmpl-1", Name: "deploy-v2", DAGJSON: `{"steps":[]}`, Scope: domain.ScopePersonal, ExpectedVersion: 99,
	})
	if err == nil {
		t.Fatal("expected an error on a stale expected_version")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Code != "WORKFLOW_TEMPLATE_VERSION_CONFLICT" {
		t.Errorf("want code WORKFLOW_TEMPLATE_VERSION_CONFLICT, got %q", ae.Code)
	}
	st, ok := status.FromError(apperrors.ToGRPCStatus(err))
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("want codes.FailedPrecondition, got %v", st.Code())
	}
}

func TestUpdateTemplate_CyclicParent_RejectsBeforeWriting(t *testing.T) {
	repo := newFakeTemplateRepository()
	// tmpl-a is root; tmpl-b's parent is tmpl-a. Attempting to set tmpl-a's
	// parent to tmpl-b would close a 2-hop cycle (a -> b -> a) — exactly the
	// case ErrTemplateSelfParent's updated doc comment says UpdateTemplate
	// must now catch via ResolveChain, since NewWorkflowTemplate's
	// direct-self-parent check alone can't see it.
	tmplA, err := domain.NewWorkflowTemplate("tmpl-a", "tenant-1", "a", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building tmpl-a: %v", err)
	}
	tmplB, err := domain.NewWorkflowTemplate("tmpl-b", "tenant-1", "b", `{"steps":[]}`, domain.ScopePersonal, "tmpl-a", "owner-1")
	if err != nil {
		t.Fatalf("building tmpl-b: %v", err)
	}
	repo.templates[tmplA.ID] = tmplA
	repo.templates[tmplB.ID] = tmplB

	uc := NewUpdateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err = uc.Execute(ctx, UpdateTemplateInput{
		ID: "tmpl-a", Name: "a", DAGJSON: `{"steps":[]}`, Scope: domain.ScopePersonal,
		ParentTemplateID: "tmpl-b", ExpectedVersion: 1,
	})
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Code != "WORKFLOW_TEMPLATE_CYCLE" {
		t.Errorf("want code WORKFLOW_TEMPLATE_CYCLE, got %q", ae.Code)
	}
	if repo.updateCalls != 0 {
		t.Errorf("cycle check must short-circuit before any write — want 0 Update calls, got %d", repo.updateCalls)
	}
}

func TestUpdateTemplate_EmptyParent_SkipsResolveChain(t *testing.T) {
	repo := newFakeTemplateRepository()
	existing, err := domain.NewWorkflowTemplate("tmpl-1", "tenant-1", "deploy", `{"steps":[]}`, domain.ScopePersonal, "", "owner-1")
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	repo.templates[existing.ID] = existing
	// If Execute called ResolveChain despite ParentTemplateID=="", this
	// sentinel error would surface — its absence is the proof it wasn't
	// called.
	repo.resolveErr = errors.New("ResolveChain should not be called when ParentTemplateID is empty")

	uc := NewUpdateTemplate(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err = uc.Execute(ctx, UpdateTemplateInput{
		ID: "tmpl-1", Name: "deploy-v2", DAGJSON: `{"steps":[]}`, Scope: domain.ScopePersonal, ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error (ResolveChain must not have been called): %v", err)
	}
}

// TestUpdateTemplate_NoExecutionRepositoryDependency is a structural
// regression guard: UpdateTemplate must never gain an ExecutionRepository
// dependency (updating a template whose id a completed execution's
// DefinitionSnapshot already references must succeed without touching
// execution state at all — DefinitionSnapshot freezes at Execute time). If
// a future edit adds such a dependency, NewUpdateTemplate's signature
// changes and this file fails to compile, which is the intended signal.
func TestUpdateTemplate_NoExecutionRepositoryDependency(t *testing.T) {
	repo := newFakeTemplateRepository()
	var _ *UpdateTemplate = NewUpdateTemplate(repo) // single-argument constructor: TemplateRepository only
}
