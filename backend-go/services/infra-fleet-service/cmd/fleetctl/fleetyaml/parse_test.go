package fleetyaml

import (
	"strings"
	"testing"
)

// sampleFleetYaml is docs/logic/fleet/BL-FLEET-01-fleet-inventory.md's
// sample schema, minus identityFile (backend-go rejects it) and with
// vaultSshRole substituted — the field this schema now maps to
// FleetServerInput.VaultSSHRole.
const sampleFleetYaml = `
defaults:
  relayGracePeriodSec: 30
  nodeVersion: "22"

projects:
  - name: backend
    tags: [production, api]
  - name: frontend
    tags: [staging]

servers:
  - hostname: dev1.example.com
    user: ubuntu
    vaultSshRole: role-backend
    project: backend
    tags: [primary, gpu]
    port: 22
  - hostname: dev2.example.com
    user: ubuntu
    vaultSshRole: role-backend
    project: backend
    tags: [secondary]
  - hostname: fe-dev.example.com
    user: ubuntu
    vaultSshRole: role-frontend
    project: frontend
`

func TestParse_SampleSchema(t *testing.T) {
	result, err := Parse([]byte(sampleFleetYaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Servers) != 3 {
		t.Fatalf("expected 3 servers, got %d: %+v", len(result.Servers), result.Servers)
	}

	want := []ServerInput{
		{Host: "dev1.example.com", User: "ubuntu", VaultSSHRole: "role-backend", Project: "backend", Tags: []string{"primary", "gpu"}},
		{Host: "dev2.example.com", User: "ubuntu", VaultSSHRole: "role-backend", Project: "backend", Tags: []string{"secondary"}},
		{Host: "fe-dev.example.com", User: "ubuntu", VaultSSHRole: "role-frontend", Project: "frontend"},
	}
	for i, w := range want {
		got := result.Servers[i]
		if got.Host != w.Host || got.User != w.User || got.VaultSSHRole != w.VaultSSHRole || got.Project != w.Project {
			t.Errorf("server %d: expected %+v, got %+v", i, w, got)
		}
	}

	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "port 22") {
		t.Errorf("expected exactly 1 warning about the dropped port, got %+v", result.Warnings)
	}
}

func TestParse_UndeclaredProjectFails(t *testing.T) {
	yaml := `
projects:
  - name: backend
servers:
  - hostname: dev1.example.com
    user: ubuntu
    vaultSshRole: role-1
    project: nonexistent
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for a server referencing an undeclared project")
	}
	if !strings.Contains(err.Error(), `project "nonexistent" is not declared`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParse_ServerIdentityFileFails(t *testing.T) {
	yaml := `
servers:
  - hostname: dev1.example.com
    user: ubuntu
    identityFile: ~/.ssh/dev_key
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for a server carrying identityFile")
	}
	if !strings.Contains(err.Error(), "identityFile is not supported against backend-go") {
		t.Errorf("expected the actionable Vault-role message, got: %v", err)
	}
}

func TestParse_DefaultsIdentityFileFails(t *testing.T) {
	yaml := `
defaults:
  identityFile: ~/.ssh/dev_key
servers:
  - hostname: dev1.example.com
    user: ubuntu
    vaultSshRole: role-1
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for defaults carrying identityFile")
	}
	if !strings.Contains(err.Error(), "identityFile is not supported against backend-go") {
		t.Errorf("expected the actionable Vault-role message, got: %v", err)
	}
}

func TestParse_PortOutOfRangeFails(t *testing.T) {
	yaml := `
servers:
  - hostname: dev1.example.com
    user: ubuntu
    vaultSshRole: role-1
    port: 70000
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for a port outside 1-65535")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParse_MissingHostnameFails(t *testing.T) {
	yaml := `
servers:
  - user: ubuntu
    vaultSshRole: role-1
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for a server missing hostname")
	}
}

func TestParse_InvalidHostnameFails(t *testing.T) {
	yaml := `
servers:
  - hostname: "not a hostname!!"
    user: ubuntu
    vaultSshRole: role-1
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for an invalid hostname")
	}
}

func TestParse_IPHostnameAccepted(t *testing.T) {
	yaml := `
servers:
  - hostname: "10.0.0.5"
    user: ubuntu
    vaultSshRole: role-1
`
	result, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Servers) != 1 || result.Servers[0].Host != "10.0.0.5" {
		t.Errorf("expected the IP address to be accepted as a hostname, got %+v", result.Servers)
	}
}

func TestParse_MissingUserFails(t *testing.T) {
	yaml := `
servers:
  - hostname: dev1.example.com
    vaultSshRole: role-1
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error for a server missing user")
	}
}

func TestParse_MissingVaultSSHRoleFails(t *testing.T) {
	yaml := `
servers:
  - hostname: dev1.example.com
    user: ubuntu
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected an error when neither the server nor defaults set vaultSshRole")
	}
}

func TestParse_DefaultsVaultSSHRoleAppliesToServersWithoutOwnRole(t *testing.T) {
	yaml := `
defaults:
  vaultSshRole: role-default
servers:
  - hostname: dev1.example.com
    user: ubuntu
  - hostname: dev2.example.com
    user: ubuntu
    vaultSshRole: role-override
`
	result, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Servers[0].VaultSSHRole != "role-default" {
		t.Errorf("expected defaults.vaultSshRole to apply, got %q", result.Servers[0].VaultSSHRole)
	}
	if result.Servers[1].VaultSSHRole != "role-override" {
		t.Errorf("expected the server's own vaultSshRole to win, got %q", result.Servers[1].VaultSSHRole)
	}
}
