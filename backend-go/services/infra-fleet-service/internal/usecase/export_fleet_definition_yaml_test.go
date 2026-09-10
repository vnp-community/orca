package usecase

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestExportFleetDefinitionYaml_RequiresTenantContext(t *testing.T) {
	uc := NewExportFleetDefinitionYaml(&fakeFleetDefinitionRepository{})
	_, err := uc.Execute(context.Background(), "def-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestExportFleetDefinitionYaml_NotFound_ReturnsNotFoundError(t *testing.T) {
	uc := NewExportFleetDefinitionYaml(&fakeFleetDefinitionRepository{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "unknown")
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}

func TestExportFleetDefinitionYaml_ProducesValidYamlStructure(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{
				{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: "ssh-role-dev"},
			},
		},
	}}
	uc := NewExportFleetDefinitionYaml(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "def-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse with a generic YAML decoder into a map to check exact field
	// names (camelCase vaultSshRole, not vault_ssh_role) — matches
	// TASK-BE-FLEET-005's verified field naming.
	var generic map[string]any
	if err := yaml.Unmarshal([]byte(out), &generic); err != nil {
		t.Fatalf("output is not valid YAML: %v\n%s", err, out)
	}
	servers, ok := generic["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("expected 1 server entry, got %+v", generic["servers"])
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected server entry to be a map, got %T", servers[0])
	}
	for _, field := range []string{"id", "label", "host", "username", "vaultSshRole"} {
		if _, present := server[field]; !present {
			t.Errorf("expected field %q in exported YAML server entry, got %+v", field, server)
		}
	}
	if _, present := server["vault_ssh_role"]; present {
		t.Error("expected snake_case vault_ssh_role to NOT be present — field name must be camelCase vaultSshRole")
	}
	if server["host"] != "10.0.0.1" || server["username"] != "orca" || server["vaultSshRole"] != "ssh-role-dev" {
		t.Errorf("unexpected field values: %+v", server)
	}
}

// TestExportFleetDefinitionYaml_RoundTrip_MatchesFleetConfigSchema checks
// the exported YAML's field set against FleetServerSchema's REAL field
// list, as it exists today (fleet-config-parser.ts) — id/label/host/
// username are all present in both. `vaultSshRole` is intentionally NOT
// checked against FleetServerSchema's real field list here: as of this
// task, FleetServerSchema has no `vaultSshRole` field (TASK-BE-FLEET-005,
// which would add it, was skipped — out of scope, frontend/). This usecase
// still emits it per this task's own spec, but the true cross-language
// round-trip (export -> TS parse -> equivalent FleetSpec) does not fully
// hold today — see export_fleet_definition_yaml.go's doc comment. This test
// documents the CURRENTLY-VERIFIABLE overlap, not a false claim of full
// round-trip.
func TestExportFleetDefinitionYaml_RoundTrip_MatchesFleetConfigSchema(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{{Host: "10.0.0.1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	uc := NewExportFleetDefinitionYaml(repo)
	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "def-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var generic map[string]any
	if err := yaml.Unmarshal([]byte(out), &generic); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	servers := generic["servers"].([]any)
	server := servers[0].(map[string]any)
	// FleetServerSchema's REQUIRED fields (fleet-config-parser.ts): id,
	// label, host — a re-import would fail Zod validation without these.
	for _, required := range []string{"id", "label", "host"} {
		if _, present := server[required]; !present {
			t.Errorf("FleetServerSchema requires %q — missing from exported YAML: %+v", required, server)
		}
	}
}

func TestExportFleetDefinitionYaml_NoProvision_OmitsProvisionField(t *testing.T) {
	repo := &fakeFleetDefinitionRepository{byID: map[string]domain.FleetDefinition{
		fleetDefKey("tenant-1", "def-1"): {
			ID: "def-1", TenantID: "tenant-1", Name: "fleet-1",
			Servers:   []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
			Provision: nil,
		},
	}}
	uc := NewExportFleetDefinitionYaml(repo)
	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, "def-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "provision") {
		t.Errorf("expected no provision key when def.Provision is nil, got:\n%s", out)
	}
}
