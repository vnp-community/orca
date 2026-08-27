# TASK-SSH-01-04: Persist port/known-hosts/jump-host in `SshTargetStore`

**From Solution:** SOL-SSH-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-SSH-01-02, TASK-SSH-01-03
**Status:** `[ ]` TODO

---

## Context

`SshTargetStore.Create`/`List`/`Get` currently only read/write
`id, tenant_id, host, user_name, vault_ssh_role`. Once `domain.SshTarget`
(TASK-SSH-01-02) and the `ssh_targets` columns (TASK-SSH-01-03) exist, the
repository needs to actually persist and load them — otherwise every new
field silently zeroes out on save/read.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`,
update `SshTargetStore.Create`:

```go
func (s *SshTargetStore) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	var jumpHostID *string
	if target.JumpHostTargetID != "" {
		jumpHostID = &target.JumpHostTargetID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.ssh_targets (id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, target.ID, target.TenantID, target.Host, target.Port, target.UserName, target.VaultSSHRole, target.KnownHostsFingerprint, jumpHostID)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: insert ssh target: %w", err)
	}
	return target, nil
}
```

Update `List`:

```go
func (s *SshTargetStore) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id
		FROM infra.ssh_targets
		WHERE tenant_id = $1
		ORDER BY host
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ssh targets: %w", err)
	}
	defer rows.Close()

	var out []domain.SshTarget
	for rows.Next() {
		var t domain.SshTarget
		var jumpHostID *string
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Host, &t.Port, &t.UserName, &t.VaultSSHRole, &t.KnownHostsFingerprint, &jumpHostID); err != nil {
			return nil, fmt.Errorf("postgres: scan ssh target row: %w", err)
		}
		if jumpHostID != nil {
			t.JumpHostTargetID = *jumpHostID
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ssh target rows: %w", err)
	}
	return out, nil
}
```

Update `Get` the same way (same column list, same `*string` -> `""` unwrap
for `jump_host_target_id`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestSshTarget -v
```

Expected: build succeeds; a round-trip `Create` then `Get`/`List` test
(extend `repository_test.go` if no existing SSH-target coverage) returns the
same `Port`/`KnownHostsFingerprint`/`JumpHostTargetID` that was saved.
