# TASK-SSH-01-02: Add Port/KnownHostsFingerprint/JumpHostTargetID to `domain.SshTarget`

**From Solution:** SOL-SSH-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go`
**Depends on:** none
**Status:** `[x] DONE — domain.SshTarget + NewSshTarget carry Port/KnownHostsFingerprint/JumpHostTargetID, port 0 defaults to 22, tests pass`

---

## Context

`ssh_target.go`'s own doc comment (line 22-24) already flags port,
known-hosts fingerprint, and jump-host chaining as "not modeled in this
scaffold" even though `infra-fleet-service.md` §4 already specifies them.
This task closes that gap on the domain type.

## Changes to make

Replace the current `SshTarget` struct and `NewSshTarget` constructor in
`backend-go/services/infra-fleet-service/internal/domain/ssh_target.go`:

```go
// SshTarget is host/port/username plus a pointer to the Vault SSH secrets
// engine role that signs short-lived certificates for this target — never a
// raw private key, per the security invariant in
// specs/backend-go/services/infra-fleet-service.md §9.
type SshTarget struct {
	ID                    string
	TenantID              string
	Host                  string
	Port                  int    // defaults to 22 at construction
	UserName              string
	VaultSSHRole          string
	KnownHostsFingerprint string // "" = unverified — sshconn.Connector falls back to InsecureIgnoreHostKey
	JumpHostTargetID      string // "" = no jump host; self-referential FK to another SshTarget.ID
}

// NewSshTarget constructs an SshTarget, enforcing the invariants above.
// port == 0 defaults to 22, mirroring sshconn.defaultSSHPort's existing
// default so a caller that omits port keeps today's behavior exactly.
func NewSshTarget(id, tenantID, host string, port int, userName, vaultSSHRole, knownHostsFingerprint, jumpHostTargetID string) (SshTarget, error) {
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
	if port == 0 {
		port = 22
	}
	return SshTarget{
		ID:                    id,
		TenantID:              tenantID,
		Host:                  host,
		Port:                  port,
		UserName:              userName,
		VaultSSHRole:          vaultSSHRole,
		KnownHostsFingerprint: knownHostsFingerprint,
		JumpHostTargetID:      jumpHostTargetID,
	}, nil
}
```

Update the doc comment above the struct to drop the "not modeled in this
scaffold" sentence, since it no longer applies.

`NewSshTarget`'s signature changed (2 new required-position params + 2
existing appended) — every caller in this package (`create_ssh_target.go`,
tests) needs updating; that's covered by TASK-SSH-01-05 and
TASK-SSH-01-08's test updates, not this task.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/internal/domain/... 2>&1 | head -50
```

Expected: the package itself builds; callers elsewhere in the service will
fail to compile until TASK-SSH-01-05/08 update them — that's expected at
this point in the sequence.
