package domain

import (
	"errors"
	"time"
)

// FleetSpecServer describes one server entry in a fleet spec/definition —
// moved here from usecase.FleetSpecServer (CR-FLEET-001's original
// location, TASK-BE-FLEET-001) so domain.FleetDefinition can reference it
// without domain importing usecase (see BE-FLEET-SOL-003 §2 for the
// dependency-direction rationale).
type FleetSpecServer struct {
	Host, UserName, VaultSSHRole string
	Kind                         AgentKind
}

// ProvisionConfig mirrors CR-FLEET-002's FleetConfigSchema.provision —
// nil means the definition only registers pre-existing hosts, no new
// infrastructure is created.
type ProvisionConfig struct {
	IaC        string // "terraform"
	WorkingDir string
	VarsFile   string
	// DefaultUserName/DefaultVaultSSHRole — TASK-BE-FLEET-014's chosen
	// answer to BE-FLEET-SOL-003 §4's open question: ApplyTerraformPlan's
	// TerraformInstance (TASK-BE-FLEET-006) only carries Host, but
	// CreateSshTarget needs UserName/VaultSSHRole too. Every instance
	// Terraform creates for this definition uses these 2 shared values
	// (MVP assumption: one `apply` run serves one team/purpose, so one
	// Vault role is enough) — see DeployFleetDefinition's doc comment for
	// the alternative considered and rejected (per-instance
	// vault_ssh_role in terraform output).
	DefaultUserName     string
	DefaultVaultSSHRole string
}

// FleetDefinition is CR-FLEET-003's persisted source of truth for a fleet
// — Servers/Provision are the durable record of what BulkProvisionFleet
// (CR-FLEET-001) / ApplyTerraformPlan (CR-FLEET-002) were last asked to do.
type FleetDefinition struct {
	ID        string
	TenantID  string
	Name      string
	Version   int
	Servers   []FleetSpecServer
	Provision *ProvisionConfig
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	// ErrEmptyFleetDefinitionTenant is returned when TenantID is empty.
	ErrEmptyFleetDefinitionTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyFleetDefinitionName is returned when Name is empty.
	ErrEmptyFleetDefinitionName = errors.New("domain: name is required")
	// ErrEmptyFleetDefinitionServers guards against a definition with
	// nothing to provision/register.
	ErrEmptyFleetDefinitionServers = errors.New("domain: at least one server is required")
	// ErrFleetDefinitionNotFound is returned by FleetDefinitionRepository.Get
	// when no row matches (tenantID, id) — mirrors
	// ErrEphemeralVmRuntimeNotFound's convention.
	ErrFleetDefinitionNotFound = errors.New("domain: fleet definition not found")
	// ErrFleetDefinitionVersionConflict is returned by
	// FleetDefinitionRepository.Update when the row's version has already
	// moved past the version the caller last read (optimistic locking —
	// TASK-BE-FLEET-012's chosen concurrency-control strategy, since
	// CR-FLEET-003 did not specify one).
	ErrFleetDefinitionVersionConflict = errors.New("domain: fleet definition was updated concurrently, refetch and retry")
)

// NewFleetDefinition validates required fields and returns a version-1
// FleetDefinition — mirror NewSshTarget/NewDevServer's validate-then-construct
// pattern already used throughout this package.
func NewFleetDefinition(id, tenantID, name string, servers []FleetSpecServer, provision *ProvisionConfig, createdBy string) (FleetDefinition, error) {
	if tenantID == "" {
		return FleetDefinition{}, ErrEmptyFleetDefinitionTenant
	}
	if name == "" {
		return FleetDefinition{}, ErrEmptyFleetDefinitionName
	}
	if len(servers) == 0 {
		return FleetDefinition{}, ErrEmptyFleetDefinitionServers
	}
	return FleetDefinition{
		ID: id, TenantID: tenantID, Name: name, Version: 1,
		Servers: servers, Provision: provision, CreatedBy: createdBy,
	}, nil
}
