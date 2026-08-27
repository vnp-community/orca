package domain

import "errors"

var (
	// ErrEmptySshTargetTenant is returned when TenantID is empty.
	ErrEmptySshTargetTenant = errors.New("domain: tenant_id is required")
	// ErrEmptySshTargetHost guards against an unreachable SSH target.
	ErrEmptySshTargetHost = errors.New("domain: host is required")
	// ErrEmptySshTargetUser guards against a target with no login identity.
	ErrEmptySshTargetUser = errors.New("domain: user is required")
	// ErrEmptyVaultSSHRole enforces this service's security invariant (see
	// specs/backend-go/services/infra-fleet-service.md §9): no raw SSH key
	// material is ever stored here, only a pointer into Vault's SSH secrets
	// engine role used to issue a short-lived certificate per connection.
	ErrEmptyVaultSSHRole = errors.New("domain: vault_ssh_role is required — this service never stores raw key material")
)

// SshTarget is host/port/username plus a pointer to the Vault SSH secrets
// engine role that signs short-lived certificates for this target — never a
// raw private key, per the security invariant in
// specs/backend-go/services/infra-fleet-service.md §9. Port, known-hosts
// fingerprint, and jump-host chaining from the fuller design-doc entity are
// not modeled in this scaffold — see this service's README "Known gaps".
type SshTarget struct {
	ID           string
	TenantID     string
	Host         string
	UserName     string
	VaultSSHRole string
}

// NewSshTarget constructs an SshTarget, enforcing the invariants above.
func NewSshTarget(id, tenantID, host, userName, vaultSSHRole string) (SshTarget, error) {
	if tenantID == "" {
		return SshTarget{}, ErrEmptySshTargetTenant
	}
	if host == "" {
		return SshTarget{}, ErrEmptySshTargetHost
	}
	if userName == "" {
		return SshTarget{}, ErrEmptySshTargetUser
	}
	if vaultSSHRole == "" {
		return SshTarget{}, ErrEmptyVaultSSHRole
	}
	return SshTarget{ID: id, TenantID: tenantID, Host: host, UserName: userName, VaultSSHRole: vaultSSHRole}, nil
}
