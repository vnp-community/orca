package domain

import "testing"

func TestNewFleetDefinition_RequiresTenantID(t *testing.T) {
	_, err := NewFleetDefinition("id1", "", "my-fleet", []FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}, nil, "user1")
	if err != ErrEmptyFleetDefinitionTenant {
		t.Errorf("expected ErrEmptyFleetDefinitionTenant, got %v", err)
	}
}

func TestNewFleetDefinition_RequiresName(t *testing.T) {
	_, err := NewFleetDefinition("id1", "tenant1", "", []FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}, nil, "user1")
	if err != ErrEmptyFleetDefinitionName {
		t.Errorf("expected ErrEmptyFleetDefinitionName, got %v", err)
	}
}

func TestNewFleetDefinition_RequiresAtLeastOneServer(t *testing.T) {
	_, err := NewFleetDefinition("id1", "tenant1", "my-fleet", nil, nil, "user1")
	if err != ErrEmptyFleetDefinitionServers {
		t.Errorf("expected ErrEmptyFleetDefinitionServers, got %v", err)
	}
}

func TestNewFleetDefinition_ValidInput_SetsVersion1(t *testing.T) {
	servers := []FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}}
	provision := &ProvisionConfig{IaC: "terraform", WorkingDir: "/infra", VarsFile: "prod.tfvars"}
	fd, err := NewFleetDefinition("id1", "tenant1", "my-fleet", servers, provision, "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fd.Version != 1 {
		t.Errorf("expected Version 1, got %d", fd.Version)
	}
	if fd.ID != "id1" || fd.TenantID != "tenant1" || fd.Name != "my-fleet" || fd.CreatedBy != "user1" {
		t.Errorf("unexpected fields: %+v", fd)
	}
	if len(fd.Servers) != 1 || fd.Servers[0].Host != "h1" {
		t.Errorf("unexpected servers: %+v", fd.Servers)
	}
	if fd.Provision == nil || fd.Provision.WorkingDir != "/infra" {
		t.Errorf("unexpected provision: %+v", fd.Provision)
	}
}
