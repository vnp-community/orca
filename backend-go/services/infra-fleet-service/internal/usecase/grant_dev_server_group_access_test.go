package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerGroupGrantRepository is an in-memory
// DevServerGroupGrantRepository, shared across this package's grant/
// list-for-user tests.
type fakeDevServerGroupGrantRepository struct {
	created   []domain.DevServerGroupGrant
	createErr error
	deleteErr error
	deleted   []string
	byTenant  map[string][]domain.DevServerGroupGrant
	listErr   error
}

func (f *fakeDevServerGroupGrantRepository) Create(ctx context.Context, grant domain.DevServerGroupGrant) (domain.DevServerGroupGrant, error) {
	if f.createErr != nil {
		return domain.DevServerGroupGrant{}, f.createErr
	}
	f.created = append(f.created, grant)
	if f.byTenant == nil {
		f.byTenant = map[string][]domain.DevServerGroupGrant{}
	}
	f.byTenant[grant.TenantID] = append(f.byTenant[grant.TenantID], grant)
	return grant, nil
}

func (f *fakeDevServerGroupGrantRepository) Delete(ctx context.Context, tenantID, grantID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, grantID)
	return nil
}

func (f *fakeDevServerGroupGrantRepository) ListByGroup(ctx context.Context, tenantID, groupID string) ([]domain.DevServerGroupGrant, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.DevServerGroupGrant
	for _, g := range f.byTenant[tenantID] {
		if g.DevServerGroupID == groupID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeDevServerGroupGrantRepository) ListAll(ctx context.Context, tenantID string) ([]domain.DevServerGroupGrant, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byTenant[tenantID], nil
}

func TestGrantDevServerGroupAccess_RequiresAdmin(t *testing.T) {
	uc := NewGrantDevServerGroupAccess(&fakeDevServerGroupGrantRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, GrantDevServerGroupAccessInput{
		DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
	})
	if err == nil {
		t.Fatal("expected an error for a non-admin caller")
	}
}

func TestGrantDevServerGroupAccess_CreatesGrant(t *testing.T) {
	repo := &fakeDevServerGroupGrantRepository{}
	uc := NewGrantDevServerGroupAccess(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, GrantDevServerGroupAccessInput{
		DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.DevServerGroupID != "g1" || got.GranteeID != "dept1" {
		t.Errorf("unexpected grant: %+v", got)
	}
}

func TestGrantDevServerGroupAccess_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerGroupGrantRepository{createErr: errors.New("db unavailable")}
	uc := NewGrantDevServerGroupAccess(repo)

	ctx := withAdminTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, GrantDevServerGroupAccessInput{
		DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1",
	})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
