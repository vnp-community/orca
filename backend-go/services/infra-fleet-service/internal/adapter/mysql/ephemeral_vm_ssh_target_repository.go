package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// EphemeralVmSshTargetStore implements usecase.EphemeralVmSshTargetRepository
// against ephemeral_vm_ssh_targets (migrations/mysql/0015/0016).
type EphemeralVmSshTargetStore struct {
	db *sql.DB
}

func NewEphemeralVmSshTargetStore(db *sql.DB) *EphemeralVmSshTargetStore {
	return &EphemeralVmSshTargetStore{db: db}
}

// Upsert inserts or updates the one row for record.RuntimeID (unique index
// idx_ephemeral_vm_ssh_targets_runtime) — see
// usecase.EphemeralVmSshTargetRepository's doc comment for why Provision
// re-running for the same runtime is expected, not an error case.
// ON DUPLICATE KEY UPDATE is MySQL's ON CONFLICT DO UPDATE equivalent.
func (s *EphemeralVmSshTargetStore) Upsert(ctx context.Context, record domain.EphemeralVmSshTargetRecord) (domain.EphemeralVmSshTargetRecord, error) {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ephemeral_vm_ssh_targets
			(id, tenant_id, runtime_id, host, port, username, identity_file_vault_path, identity_agent_vault_path, host_key_fingerprint, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(6))
		ON DUPLICATE KEY UPDATE
			host = VALUES(host),
			port = VALUES(port),
			username = VALUES(username),
			identity_file_vault_path = VALUES(identity_file_vault_path),
			identity_agent_vault_path = VALUES(identity_agent_vault_path),
			host_key_fingerprint = VALUES(host_key_fingerprint),
			updated_at = CURRENT_TIMESTAMP(6)
	`, record.ID, record.TenantID, record.RuntimeID, record.Host, record.Port, record.Username,
		nullableText(record.IdentityFileVaultPath), nullableText(record.IdentityAgentVaultPath), nullableText(record.HostKeyFingerprint))
	if err != nil {
		return domain.EphemeralVmSshTargetRecord{}, fmt.Errorf("mysql: upsert ephemeral vm ssh target: %w", err)
	}
	return record, nil
}

// Get fetches the hidden-target audit row for (tenantID, runtimeID).
// found=false with a nil error means "no dial has been recorded for this
// runtime yet" — not a failure the caller needs to branch on differently.
func (s *EphemeralVmSshTargetStore) Get(ctx context.Context, tenantID, runtimeID string) (domain.EphemeralVmSshTargetRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, runtime_id, host, port, username,
		       COALESCE(identity_file_vault_path, ''), COALESCE(identity_agent_vault_path, ''),
		       COALESCE(host_key_fingerprint, '')
		FROM ephemeral_vm_ssh_targets
		WHERE tenant_id = ? AND runtime_id = ?
	`, tenantID, runtimeID)

	var rec domain.EphemeralVmSshTargetRecord
	if err := row.Scan(&rec.ID, &rec.TenantID, &rec.RuntimeID, &rec.Host, &rec.Port, &rec.Username,
		&rec.IdentityFileVaultPath, &rec.IdentityAgentVaultPath, &rec.HostKeyFingerprint); err != nil {
		if err == sql.ErrNoRows {
			return domain.EphemeralVmSshTargetRecord{}, false, nil
		}
		return domain.EphemeralVmSshTargetRecord{}, false, fmt.Errorf("mysql: get ephemeral vm ssh target: %w", err)
	}
	return rec, true, nil
}

// nullableText converts an empty string to a real SQL NULL — mirrors this
// package's Postgres counterpart's identical convention.
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
