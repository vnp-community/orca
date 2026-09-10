package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeFleetDefinitionRepository is an in-memory FleetDefinitionRepository,
// shared by create/update/get/list_fleet_definition_test.go (same package).
type fakeFleetDefinitionRepository struct {
	created   []domain.FleetDefinition
	createErr error

	updateErr    error
	updateCalled []domain.FleetDefinition

	byID   map[string]domain.FleetDefinition // keyed "tenantID/id"
	getErr error

	byTenant map[string][]domain.FleetDefinition
	listErr  error
}

func fleetDefKey(tenantID, id string) string { return tenantID + "/" + id }

func (f *fakeFleetDefinitionRepository) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	if f.createErr != nil {
		return domain.FleetDefinition{}, f.createErr
	}
	f.created = append(f.created, def)
	if f.byID == nil {
		f.byID = map[string]domain.FleetDefinition{}
	}
	f.byID[fleetDefKey(def.TenantID, def.ID)] = def
	return def, nil
}

func (f *fakeFleetDefinitionRepository) Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	f.updateCalled = append(f.updateCalled, def)
	if f.updateErr != nil {
		return domain.FleetDefinition{}, f.updateErr
	}
	if f.byID == nil {
		f.byID = map[string]domain.FleetDefinition{}
	}
	f.byID[fleetDefKey(def.TenantID, def.ID)] = def
	return def, nil
}

func (f *fakeFleetDefinitionRepository) Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error) {
	if f.getErr != nil {
		return domain.FleetDefinition{}, f.getErr
	}
	def, ok := f.byID[fleetDefKey(tenantID, id)]
	if !ok {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionNotFound
	}
	return def, nil
}

func (f *fakeFleetDefinitionRepository) List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byTenant[tenantID], nil
}

func withTenantAndUser(ctx context.Context, tenantID, userID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(ctx, tenantID), userID)
}

func TestCreateFleetDefinition_RequiresTenantContext(t *testing.T) {
	uc := NewCreateFleetDefinition(&fakeFleetDefinitionRepository{})
	_, err := uc.Execute(context.Background(), CreateFleetDefinitionInput{
		Name:    "fleet-1",
		Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateFleetDefinition_ValidatesInput(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{}
	uc := NewCreateFleetDefinition(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateFleetDefinitionInput{Name: "", Servers: nil})
	if err == nil {
		t.Fatal("expected an error for empty name/servers")
	}
	if len(repo.created) != 0 {
		t.Error("expected no creation to occur for invalid input")
	}
}

func TestCreateFleetDefinition_CreatesWithTenantFromContext(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{}
	uc := NewCreateFleetDefinition(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateFleetDefinitionInput{
		Name:    "fleet-1",
		Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.CreatedBy != "user-1" {
		t.Errorf("expected tenant/user from context, got %+v", got)
	}
	if got.ID == "" || got.Version != 1 {
		t.Errorf("expected generated ID and version 1, got %+v", got)
	}
}

func TestCreateFleetDefinition_RequiresUserContext(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{}
	uc := NewCreateFleetDefinition(repo)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1") // no user id
	_, err := uc.Execute(ctx, CreateFleetDefinitionInput{
		Name:    "fleet-1",
		Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err == nil {
		t.Fatal("expected an error when no user is in context")
	}
}

func TestCreateFleetDefinition_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{createErr: errors.New("db unavailable")}
	uc := NewCreateFleetDefinition(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateFleetDefinitionInput{
		Name:    "fleet-1",
		Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
	})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
