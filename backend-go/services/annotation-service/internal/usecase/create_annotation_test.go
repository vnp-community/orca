package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// fakeRepository is an in-memory Repository — the "test against fakes, not
// a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeRepository struct {
	created   []domain.Annotation
	createErr error

	getErr    error
	updateErr error
	deleteErr error

	byID map[string]domain.Annotation
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byID: make(map[string]domain.Annotation)}
}

// fakeOPAClient is an in-memory usecase.OPAClient stand-in for
// UpdateAnnotation/DeleteAnnotation's author-only-edit tests — avoids
// spinning up the real embedded OPA evaluator (common/policy) for unit
// tests, matching the fakeRepository pattern above.
type fakeOPAClient struct {
	allow bool
	err   error
	calls []fakeOPADecisionCall
}

type fakeOPADecisionCall struct {
	actorID, authorID, actorRole string
}

func newFakeOPAClient(allow bool) *fakeOPAClient {
	return &fakeOPAClient{allow: allow}
}

func (f *fakeOPAClient) Decision(ctx context.Context, actorID, authorID, actorRole string) (bool, error) {
	f.calls = append(f.calls, fakeOPADecisionCall{actorID: actorID, authorID: authorID, actorRole: actorRole})
	if f.err != nil {
		return false, f.err
	}
	return f.allow, nil
}

func (f *fakeRepository) CreateAnnotation(ctx context.Context, a domain.Annotation) (domain.Annotation, error) {
	if f.createErr != nil {
		return domain.Annotation{}, f.createErr
	}
	f.created = append(f.created, a)
	if f.byID == nil {
		f.byID = make(map[string]domain.Annotation)
	}
	f.byID[a.ID] = a
	return a, nil
}

func (f *fakeRepository) ListAnnotations(ctx context.Context, tenantID, repoID, filePath, pageToken string, pageSize int32) ([]domain.Annotation, string, error) {
	var out []domain.Annotation
	for _, a := range f.created {
		if a.TenantID == tenantID && a.Anchor.RepoID == repoID {
			out = append(out, a)
		}
	}
	return out, "", nil
}

func (f *fakeRepository) GetAnnotation(ctx context.Context, tenantID, id string) (domain.Annotation, error) {
	if f.getErr != nil {
		return domain.Annotation{}, f.getErr
	}
	a, ok := f.byID[id]
	if !ok || a.TenantID != tenantID {
		return domain.Annotation{}, domain.ErrAnnotationNotFound
	}
	return a, nil
}

func (f *fakeRepository) UpdateAnnotation(ctx context.Context, tenantID, id, content string, resolved bool) (domain.Annotation, error) {
	if f.updateErr != nil {
		return domain.Annotation{}, f.updateErr
	}
	a, ok := f.byID[id]
	if !ok || a.TenantID != tenantID {
		return domain.Annotation{}, domain.ErrAnnotationNotFound
	}
	a.Content = content
	a.Resolved = resolved
	f.byID[id] = a
	return a, nil
}

func (f *fakeRepository) DeleteAnnotation(ctx context.Context, tenantID, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	a, ok := f.byID[id]
	if !ok || a.TenantID != tenantID {
		return domain.ErrAnnotationNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeRepository) FindByRequestID(ctx context.Context, tenantID, requestID string) (domain.Annotation, bool, error) {
	for _, a := range f.created {
		if a.TenantID == tenantID && a.RequestID == requestID {
			return a, true, nil
		}
	}
	return domain.Annotation{}, false, nil
}

func withIdentity(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

func TestCreateAnnotation_RequiresTenantContext(t *testing.T) {
	uc := NewCreateAnnotation(newFakeRepository())
	_, err := uc.Execute(context.Background(), CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "hi",
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateAnnotation_RequiresUserContext(t *testing.T) {
	uc := NewCreateAnnotation(newFakeRepository())
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "hi",
	})
	if err == nil {
		t.Fatal("expected an error when no user is in context")
	}
}

func TestCreateAnnotation_RejectsInvalidAnchor(t *testing.T) {
	uc := NewCreateAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "", FilePath: "main.go", Line: 1, Content: "hi", RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected an error for empty repo_id")
	}
}

func TestCreateAnnotation_RequiresRequestID(t *testing.T) {
	uc := NewCreateAnnotation(newFakeRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "hi",
	})
	if err == nil {
		t.Fatal("expected an error when request_id is empty")
	}
}

func TestCreateAnnotation_SavesWithTenantAndAuthorFromContext(t *testing.T) {
	repo := newFakeRepository()
	uc := NewCreateAnnotation(repo)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID:    "repo-1",
		FilePath:  "main.go",
		Line:      42,
		Ref:       "abc123",
		Content:   "nit: rename this",
		RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.AuthorID != "user-1" {
		t.Errorf("expected tenant/author to come from context, got tenant=%s author=%s", got.TenantID, got.AuthorID)
	}
	if got.ID == "" {
		t.Error("expected a generated ID")
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 created annotation, got %d", len(repo.created))
	}
}

func TestCreateAnnotation_RepositoryFailurePropagates(t *testing.T) {
	repo := newFakeRepository()
	repo.createErr = errors.New("db unavailable")
	uc := NewCreateAnnotation(repo)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "hi", RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}

// TestCreateAnnotation_IdempotentRetrySameRequestIDReturnsExisting mirrors
// automation-service.RunNow's idempotency test: a retried create for the
// same (tenant_id, request_id) must return the original annotation, not
// insert a duplicate.
func TestCreateAnnotation_IdempotentRetrySameRequestIDReturnsExisting(t *testing.T) {
	repo := newFakeRepository()
	uc := NewCreateAnnotation(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	first, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "original", RequestID: "req-dup",
	})
	if err != nil {
		t.Fatalf("unexpected error on first create: %v", err)
	}

	second, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "a different body — must be ignored", RequestID: "req-dup",
	})
	if err != nil {
		t.Fatalf("unexpected error on retried create: %v", err)
	}
	if second.ID != first.ID || second.Content != first.Content {
		t.Errorf("expected the retry to return the original annotation unchanged, got %+v (original %+v)", second, first)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected exactly 1 row persisted despite the retry, got %d", len(repo.created))
	}
}

// TestCreateAnnotation_DifferentRequestIDsCreateDistinctAnnotations asserts
// the idempotency key is scoped correctly — two different request_ids
// (same tenant/anchor) must not collide.
func TestCreateAnnotation_DifferentRequestIDsCreateDistinctAnnotations(t *testing.T) {
	repo := newFakeRepository()
	uc := NewCreateAnnotation(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	first, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "a", RequestID: "req-a",
	})
	if err != nil {
		t.Fatalf("unexpected error creating first: %v", err)
	}
	second, err := uc.Execute(ctx, CreateAnnotationInput{
		RepoID: "repo-1", FilePath: "main.go", Line: 1, Content: "b", RequestID: "req-b",
	})
	if err != nil {
		t.Fatalf("unexpected error creating second: %v", err)
	}
	if first.ID == second.ID {
		t.Error("expected distinct request_ids to produce distinct annotations")
	}
	if len(repo.created) != 2 {
		t.Fatalf("expected 2 rows persisted, got %d", len(repo.created))
	}
}
