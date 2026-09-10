package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// orderTracingTerraformRunner is a TerraformRunner that appends to a shared
// trace on Apply — used only by
// TestDeployFleetDefinition_ProvisionNotNil_CallsApplyTerraformPlanBeforeBulkProvisionFleet
// to prove call ordering directly.
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

// orderTracingProvisioner wraps fakeProvisioner's Provision with the same
// shared trace, for the same ordering test.
type orderTracingProvisioner struct {
	*fakeProvisioner
	trace *[]string
}

func (p *orderTracingProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (HandshakeInfo, bool, error) {
	*p.trace = append(*p.trace, "provision:"+devServer.Host)
	return p.fakeProvisioner.Provision(ctx, devServer)
}

func newDeployFleetDefinitionForTest(fleetRepo *fakeFleetDefinitionRepository, controlDevRepo *fakeDevServerRepository, runner *fakeTerraformRunner, sshRepo *fakeSshTargetRepository, devRepo *fakeDevServerRepository, provisioner *fakeProvisioner) *DeployFleetDefinition {
	applyTerraformPlan := NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := NewBulkProvisionFleet(sshRepo, devRepo, provisioner)
	return NewDeployFleetDefinition(fleetRepo, sshRepo, applyTerraformPlan, bulkProvisionFleet)
}

func TestDeployFleetDefinition_NotFound_ReturnsNotFoundError(t *testing.T) {
	uc := newDeployFleetDefinitionForTest(&fakeFleetDefinitionRepository{}, &fakeDevServerRepository{}, &fakeTerraformRunner{}, &fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeProvisioner{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "unknown"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestDeployFleetDefinition_ProvisionNil_UpsertsThenBulkProvisions(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: nil,
		},
	}}
	sshRepo := &fakeSshTargetRepository{}
	devRepo := &fakeDevServerRepository{}
	provisioner := &fakeProvisioner{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, sshRepo, devRepo, provisioner)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success != 1 || len(result.Outcomes) != 1 || result.Outcomes[0].Host != "h1" {
		t.Fatalf("expected 1 successful outcome for h1, got %+v", result)
	}
	if len(sshRepo.upserted) != 1 || sshRepo.upserted[0].Host != "h1" || sshRepo.upserted[0].Project != "fleetdef:def-1" {
		t.Fatalf("expected h1 upserted with project fleetdef:def-1, got %+v", sshRepo.upserted)
	}
	if len(devRepo.registered) != 1 {
		t.Errorf("expected exactly 1 dev server registered, got %d", len(devRepo.registered))
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
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, &fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeProvisioner{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}) // no ControlDevServerID
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
	sshRepo := &fakeSshTargetRepository{}
	devRepo := &fakeDevServerRepository{}
	provisioner := &orderTracingProvisioner{fakeProvisioner: &fakeProvisioner{}, trace: &trace}

	applyTerraformPlan := NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := NewBulkProvisionFleet(sshRepo, devRepo, provisioner)
	uc := NewDeployFleetDefinition(fleetRepo, sshRepo, applyTerraformPlan, bulkProvisionFleet)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace) != 2 || trace[0] != "terraform" || trace[1] != "provision:10.0.0.1" {
		t.Errorf("expected terraform to run before provision, got trace: %v", trace)
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
	sshRepo := &fakeSshTargetRepository{}
	devRepo := &fakeDevServerRepository{}
	provisioner := &fakeProvisioner{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, controlDevRepo, runner, sshRepo, devRepo, provisioner)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 3 || result.Success != 3 {
		t.Fatalf("expected 1 existing + 2 terraform-created = 3 successful outcomes, got %+v", result)
	}
	hosts := map[string]bool{}
	for _, o := range result.Outcomes {
		hosts[o.Host] = true
	}
	for _, want := range []string{"existing-1", "tf-1", "tf-2"} {
		if !hosts[want] {
			t.Errorf("expected host %q in outcomes, got %+v", want, result.Outcomes)
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
	sshRepo := &fakeSshTargetRepository{}
	devRepo := &fakeDevServerRepository{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, controlDevRepo, runner, sshRepo, devRepo, &fakeProvisioner{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1", ControlDevServerID: "control-1"})
	if err == nil {
		t.Fatal("expected the terraform failure to propagate")
	}
	if len(devRepo.registered) != 0 || len(sshRepo.upserted) != 0 {
		t.Errorf("expected BulkProvisionFleet to never run (0 registrations/upserts), got %d registered, %d upserted", len(devRepo.registered), len(sshRepo.upserted))
	}
}

func TestDeployFleetDefinition_RunTwice_SameID_ReUpsertsSameHost(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: nil,
		},
	}}
	sshRepo := &fakeSshTargetRepository{}
	devRepo := &fakeDevServerRepository{}
	provisioner := &fakeProvisioner{}
	uc := newDeployFleetDefinitionForTest(fleetRepo, &fakeDevServerRepository{}, &fakeTerraformRunner{}, sshRepo, devRepo, provisioner)
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"}); err != nil {
		t.Fatalf("first run: unexpected error: %v", err)
	}
	result, err := uc.Execute(ctx, DeployFleetDefinitionInput{FleetDefinitionID: "def-1"})
	if err != nil {
		t.Fatalf("second run: unexpected error: %v", err)
	}
	// Upsert (not Create) means a re-deploy of the same host never hits a
	// unique-constraint error — both runs report success for h1.
	if result.Success != 1 || len(result.Outcomes) != 1 || result.Outcomes[0].Host != "h1" {
		t.Fatalf("expected h1 to succeed again on re-run, got %+v", result)
	}
	if len(sshRepo.upserted) != 2 {
		t.Errorf("expected 2 Upsert calls (one per run) for the same host, got %d", len(sshRepo.upserted))
	}
}
