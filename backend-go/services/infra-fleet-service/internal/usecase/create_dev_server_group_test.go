package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerGroupRepository is an in-memory DevServerGroupRepository.
type fakeDevServerGroupRepository struct {
	created   []domain.DevServerGroup
	createErr error
	byTenant  map[string][]domain.DevServerGroup
	listErr   error
}

func (f *fakeDevServerGroupRepository) Create(ctx context.Context, group domain.DevServerGroup) (domain.DevServerGroup, error) {
	if f.createErr != nil {
		return domain.DevServerGroup{}, f.createErr
	}
	f.created = append(f.created, group)
	return group, nil
}

func (f *fakeDevServerGroupRepository) List(ctx context.Context, tenantID string) ([]domain.DevServerGroup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byTenant[tenantID], nil
}

func TestCreateDevServerGroup_RequiresTenantContext(t *testing.T) {
	uc := NewCreateDevServerGroup(&fakeDevServerGroupRepository{})
	_, err := uc.Execute(context.Background(), CreateDevServerGroupInput{Name: "Backend Team"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateDevServerGroup_ValidatesInput(t *testing.T) {
	repo := &fakeDevServerGroupRepository{}
	uc := NewCreateDevServerGroup(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateDevServerGroupInput{Name: ""})
	if err == nil {
		t.Fatal("expected an error for an empty name")
	}
	if len(repo.created) != 0 {
		t.Error("expected no group to be created for invalid input")
	}
}

func TestCreateDevServerGroup_CreatesWithTenantFromContext(t *testing.T) {
	repo := &fakeDevServerGroupRepository{}
	uc := NewCreateDevServerGroup(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateDevServerGroupInput{Name: "Backend Team"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("expected tenant to come from context, got %q", got.TenantID)
	}
	if got.ID == "" {
		t.Error("expected a generated ID")
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 created group, got %d", len(repo.created))
	}
}

// TestCreateDevServerGroup_RequiresAdmin guards CR-DS-006 Phase 2's
// admin-gating — a non-admin (or role-absent, pre-Phase-2-plumbing) caller
// must be denied, never implicitly allowed.
func TestCreateDevServerGroup_RequiresAdmin(t *testing.T) {
	repo := &fakeDevServerGroupRepository{}
	uc := NewCreateDevServerGroup(repo)

	ctx := withTenant(context.Background(), "tenant-1") // no admin role
	_, err := uc.Execute(ctx, CreateDevServerGroupInput{Name: "Backend Team"})
	if err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
	if len(repo.created) != 0 {
		t.Error("expected no group to be created for a denied caller")
	}
}

func TestCreateDevServerGroup_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerGroupRepository{createErr: errors.New("db unavailable")}
	uc := NewCreateDevServerGroup(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateDevServerGroupInput{Name: "Backend Team"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
