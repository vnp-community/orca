package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerRepository is an in-memory DevServerRepository.
type fakeDevServerRepository struct {
	registered  []domain.DevServer
	registerErr error
	byID        map[string]domain.DevServer
	getErr      error
	listErr     error

	// found/bySshTarget/findErr drive FindBySshTarget's fake answer — used
	// by get_ssh_state_test.go/establish_connection_test.go.
	found          bool
	bySshTarget    domain.DevServer
	findErr        error
	registerCalled bool
	lastRegistered domain.DevServer
}

func (f *fakeDevServerRepository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	f.registerCalled = true
	f.lastRegistered = ds
	if f.registerErr != nil {
		return domain.DevServer{}, f.registerErr
	}
	f.registered = append(f.registered, ds)
	return ds, nil
}

// FindBySshTarget implements usecase.DevServerRepository.FindBySshTarget.
func (f *fakeDevServerRepository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	if f.findErr != nil {
		return domain.DevServer{}, false, f.findErr
	}
	if !f.found {
		return domain.DevServer{}, false, nil
	}
	return f.bySshTarget, true, nil
}

func (f *fakeDevServerRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	if f.getErr != nil {
		return domain.DevServer{}, f.getErr
	}
	return f.byID[id], nil
}

func (f *fakeDevServerRepository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.DevServer
	for _, ds := range f.byID {
		if ds.TenantID == tenantID {
			out = append(out, ds)
		}
	}
	return out, nil
}

func TestRegisterDevServer_RequiresTenantContext(t *testing.T) {
	uc := NewRegisterDevServer(&fakeDevServerRepository{})
	_, err := uc.Execute(context.Background(), RegisterDevServerInput{Host: "10.0.0.1", Mode: domain.ConnectionModeRelaySSH})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRegisterDevServer_ValidatesInput(t *testing.T) {
	repo := &fakeDevServerRepository{}
	uc := NewRegisterDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RegisterDevServerInput{Host: "", Mode: domain.ConnectionModeRelaySSH})
	if err == nil {
		t.Fatal("expected an error for an empty host")
	}
	if len(repo.registered) != 0 {
		t.Error("expected no registration to occur for invalid input")
	}
}

func TestRegisterDevServer_RegistersWithTenantFromContext(t *testing.T) {
	repo := &fakeDevServerRepository{}
	uc := NewRegisterDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RegisterDevServerInput{Host: "10.0.0.1", Mode: domain.ConnectionModeDirectWebSocket})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("expected tenant to come from context, got %q", got.TenantID)
	}
	if got.ID == "" {
		t.Error("expected a generated ID")
	}
	if len(repo.registered) != 1 {
		t.Fatalf("expected 1 registered dev server, got %d", len(repo.registered))
	}
}

// TestRegisterDevServer_RelaySSHRequiresSSHTargetID is the usecase-level
// regression for domain.ErrMissingSSHTargetForRelaySSH — the RPC boundary
// must reject the same invalid state the domain constructor does, not just
// the domain package's own unit tests.
func TestRegisterDevServer_RelaySSHRequiresSSHTargetID(t *testing.T) {
	repo := &fakeDevServerRepository{}
	uc := NewRegisterDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RegisterDevServerInput{Host: "10.0.0.1", Mode: domain.ConnectionModeRelaySSH})
	if err == nil {
		t.Fatal("expected an error when relay-ssh mode has no SSHTargetID")
	}
	if len(repo.registered) != 0 {
		t.Error("expected no registration to occur for invalid input")
	}
}

func TestRegisterDevServer_PassesThroughSSHTargetID(t *testing.T) {
	repo := &fakeDevServerRepository{}
	uc := NewRegisterDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RegisterDevServerInput{Host: "10.0.0.1", Mode: domain.ConnectionModeRelaySSH, SSHTargetID: "ssht-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SSHTargetID != "ssht-1" {
		t.Errorf("expected SSHTargetID to pass through, got %q", got.SSHTargetID)
	}
}

func TestRegisterDevServer_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeDevServerRepository{registerErr: errors.New("db unavailable")}
	uc := NewRegisterDevServer(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RegisterDevServerInput{Host: "10.0.0.1", Mode: domain.ConnectionModeRelaySSH, SSHTargetID: "ssht-1"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
