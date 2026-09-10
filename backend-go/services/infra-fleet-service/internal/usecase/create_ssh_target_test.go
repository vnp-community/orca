package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeSshTargetRepository is an in-memory SshTargetRepository.
type fakeSshTargetRepository struct {
	created   []domain.SshTarget
	createErr error
	byID      map[string]domain.SshTarget
	getErr    error

	// single, when its ID is non-empty, is returned by Get regardless of
	// tenantID/id — a test convenience for establish_connection_test.go's
	// single-target fixtures.
	single domain.SshTarget

	// targets (tenantID -> targets) and listErr drive List's fake answer —
	// used by list_ssh_targets_test.go.
	targets map[string][]domain.SshTarget
	listErr error

	// byHostUser (tenantID|host|user -> target) and getByHostUserErr drive
	// GetByHostUser's fake answer — used by import_fleet_inventory_test.go's
	// dry-run assertions.
	byHostUser       map[string]domain.SshTarget
	getByHostUserErr error

	// upserted records every Upsert call in order, upsertErr forces an
	// error, and upsertUpdated forces the returned `updated` flag when a
	// matching byHostUser entry does not already decide it.
	upserted      []domain.SshTarget
	upsertErr     error
	upsertUpdated bool
}

func sshTargetHostUserKey(tenantID, host, userName string) string {
	return tenantID + "|" + host + "|" + userName
}

// Upsert implements usecase.SshTargetRepository.Upsert.
func (f *fakeSshTargetRepository) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
	if f.upsertErr != nil {
		return domain.SshTarget{}, false, f.upsertErr
	}
	key := sshTargetHostUserKey(target.TenantID, target.Host, target.UserName)
	_, existed := f.byHostUser[key]
	if f.byHostUser == nil {
		f.byHostUser = map[string]domain.SshTarget{}
	}
	f.byHostUser[key] = target
	f.upserted = append(f.upserted, target)
	return target, existed || f.upsertUpdated, nil
}

// GetByHostUser implements usecase.SshTargetRepository.GetByHostUser.
func (f *fakeSshTargetRepository) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
	if f.getByHostUserErr != nil {
		return domain.SshTarget{}, false, f.getByHostUserErr
	}
	t, ok := f.byHostUser[sshTargetHostUserKey(tenantID, host, userName)]
	return t, ok, nil
}

func (f *fakeSshTargetRepository) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	if f.createErr != nil {
		return domain.SshTarget{}, f.createErr
	}
	f.created = append(f.created, target)
	return target, nil
}

func (f *fakeSshTargetRepository) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	if f.getErr != nil {
		return domain.SshTarget{}, f.getErr
	}
	if f.single.ID != "" {
		return f.single, nil
	}
	return f.byID[id], nil
}

// List implements usecase.SshTargetRepository.List.
func (f *fakeSshTargetRepository) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.targets[tenantID], nil
}

func TestCreateSshTarget_RequiresTenantContext(t *testing.T) {
	uc := NewCreateSshTarget(&fakeSshTargetRepository{})
	_, err := uc.Execute(context.Background(), CreateSshTargetInput{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: "role"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateSshTarget_ValidatesInput(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewCreateSshTarget(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateSshTargetInput{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: ""})
	if err == nil {
		t.Fatal("expected an error for a missing vault_ssh_role")
	}
	if len(repo.created) != 0 {
		t.Error("expected no creation to occur for invalid input")
	}
}

func TestCreateSshTarget_CreatesWithTenantFromContext(t *testing.T) {
	repo := &fakeSshTargetRepository{}
	uc := NewCreateSshTarget(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateSshTargetInput{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: "ssh-role-dev"})
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
		t.Fatalf("expected 1 created ssh target, got %d", len(repo.created))
	}
}

func TestCreateSshTarget_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeSshTargetRepository{createErr: errors.New("db unavailable")}
	uc := NewCreateSshTarget(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateSshTargetInput{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: "ssh-role-dev"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
