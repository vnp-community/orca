package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// orderTracingTerraformRunner is a usecase.TerraformRunner that appends to a
// shared trace on Apply — used only by
// TestDeployFleetDefinition_ProvisionNotNil_CallsApplyTerraformPlanBeforeBulkProvisionFleet
// to prove call ordering directly (not just indirectly via
// ApplyTerraformPlanFails_DoesNotCallBulkProvisionFleet).
type orderTracingTerraformRunner struct {
	trace      *[]string
	outputJSON string
	err        error
}

func (r *orderTracingTerraformRunner) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	*r.trace = append(*r.trace, "terraform")
	if r.err != nil {
		return "", r.err
	}
	return r.outputJSON, nil
}

// orderTracingDevServerRepository wraps bulkFakeDevServerRepository's
// Register with the same shared trace, for the same ordering test.
type orderTracingDevServerRepository struct {
	*bulkFakeDevServerRepository
	trace *[]string
}

func (r *orderTracingDevServerRepository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	*r.trace = append(*r.trace, "register:"+ds.Host)
	return r.bulkFakeDevServerRepository.Register(ctx, ds)
}

func newDeployFleetDefinitionForTest(fleetRepo *fakeFleetDefinitionRepository, controlDevRepo *fakeDevServerRepository, runner *fakeTerraformRunner, sshRepo *bulkFakeSshTargetRepository, devRepo *bulkFakeDevServerRepository) *DeployFleetDefinition {
	applyTerraformPlan := NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := NewBulkProvisionFleet(NewCreateSshTarget(sshRepo), NewRegisterDevServer(devRepo), NewDeleteSshTarget(sshRepo))
	return NewDeployFleetDefinition(fleetRepo, applyTerraformPlan, bulkProvisionFleet)
}

func TestDeployFleetDefinition_NotFound_ReturnsNotFoundError(t *testing.T) {
	uc := newDeployFleetDefinitionForTest(&fakeFleetDefinitionRepository{}, &fakeDevServerRepository{}, &fakeTerraformRunner{}, &bulkFakeSshTargetRepository{}, &bulkFakeDevServerRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "unknown"}, nil)
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestDeployFleetDefinition_ProvisionNil_BehavesLikeBulkProvisionFleetDirectly(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: nil,
		},
	}}
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, sshRepo, devRepo)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Status != "SUCCEEDED" || result.Results[0].Host != "h1" {
		t.Fatalf("expected 1 SUCCEEDED result for h1, got %+v", result.Results)
	}
	if len(devRepo.registered) != 1 {
		t.Errorf("expected exactly 1 dev server registered (the pre-existing one), got %d", len(devRepo.registered))
	}
}

func TestDeployFleetDefinition_ProvisionNotNil_MissingControlDevServerID_ReturnsInvalidArgument(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: &domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra"},
		},
	}}
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, &bulkFakeSshTargetRepository{}, &bulkFakeDevServerRepository{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}, nil) // no ControlDevServerID
	if err == nil {
		t.Fatal("expected an error when Provision != nil and ControlDevServerID is empty")
	}
}

func TestDeployFleetDefinition_ProvisionNotNil_CallsApplyTerraformPlanBeforeBulkProvisionFleet(t *testing.T) {
	var trace []string
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Provision: &domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra", DefaultUserName: "orca", DefaultVaultSSHRole: "role"},
		},
	}}
	controlDevRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"control-1": {ID: "control-1"}}}
	runner := &orderTracingTerraformRunner{trace: &trace, outputJSON: `{"instance_hosts":{"value":["10.0.0.1"]}}`}
	devRepo := &orderTracingDevServerRepository{bulkFakeDevServerRepository: &bulkFakeDevServerRepository{}, trace: &trace}

	applyTerraformPlan := NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := NewBulkProvisionFleet(NewCreateSshTarget(&bulkFakeSshTargetRepository{}), NewRegisterDevServer(devRepo), NewDeleteSshTarget(&bulkFakeSshTargetRepository{}))
	uc := NewDeployFleetDefinition(fleetRepo, applyTerraformPlan, bulkProvisionFleet)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace) != 2 || trace[0] != "terraform" || trace[1] != "register:10.0.0.1" {
		t.Errorf("expected terraform to run before register, got trace: %v", trace)
	}
}

func TestDeployFleetDefinition_MixedServers_ExistingAndTerraformCreated(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "existing-1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: &domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra", DefaultUserName: "orca", DefaultVaultSSHRole: "role"},
		},
	}}
	controlDevRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"control-1": {ID: "control-1"}}}
	runner := &fakeTerraformRunner{outputJSON: `{"instance_hosts":{"value":["tf-1","tf-2"]}}`}
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, controlDevRepo, runner, sshRepo, devRepo)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected 1 existing + 2 terraform-created = 3 results, got %d: %+v", len(result.Results), result.Results)
	}
	hosts := map[string]bool{}
	for _, r := range result.Results {
		hosts[r.Host] = true
		if r.Status != "SUCCEEDED" {
			t.Errorf("host %q: expected SUCCEEDED, got %q", r.Host, r.Status)
		}
	}
	for _, want := range []string{"existing-1", "tf-1", "tf-2"} {
		if !hosts[want] {
			t.Errorf("expected host %q in results, got %+v", want, result.Results)
		}
	}
}

func TestDeployFleetDefinition_ApplyTerraformPlanFails_DoesNotCallBulkProvisionFleet(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "existing-1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: &domain.ProvisionConfig{IaC: "terraform", WorkingDir: "/infra"},
		},
	}}
	controlDevRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"control-1": {ID: "control-1"}}}
	runner := &fakeTerraformRunner{applyErr: errors.New("terraform apply failed")}
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, controlDevRepo, runner, sshRepo, devRepo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"}, nil)
	if err == nil {
		t.Fatal("expected the terraform failure to propagate")
	}
	if len(devRepo.registered) != 0 || len(sshRepo.created) != 0 {
		t.Errorf("expected BulkProvisionFleet to never run (0 registrations/ssh targets), got %d registered, %d ssh targets", len(devRepo.registered), len(sshRepo.created))
	}
}

func TestDeployFleetDefinition_RunTwice_SameID_NoIdempotencyRegression(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: nil,
		},
	}}
	sshRepo := &bulkFakeSshTargetRepository{}
	devRepo := &bulkFakeDevServerRepository{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, sshRepo, devRepo)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}, nil); err != nil {
		t.Fatalf("first run: unexpected error: %v", err)
	}

	// Simulate migration 0017's (tenant_id, host) unique constraint kicking
	// in on the 2nd run for the same host — the exact *pgconn.PgError shape
	// isUniqueViolation (bulk_provision_fleet.go) checks for, same fixture
	// TestBulkProvisionFleet_DuplicateHost_ReturnsAlreadyExistsNotFailed uses.
	sshRepo.errForHost = map[string]error{"h1": &pgconn.PgError{Code: "23505", Message: "duplicate key"}}

	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}, nil)
	if err != nil {
		t.Fatalf("second run: unexpected error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Status != "SUCCEEDED" || result.Results[0].Error != "already_exists" {
		t.Fatalf("expected {Status: SUCCEEDED, Error: already_exists} on re-run, got %+v", result.Results)
	}
	if len(devRepo.registered) != 1 {
		t.Errorf("expected only the first run's registration to exist (no duplicate dev server), got %d", len(devRepo.registered))
	}
}
