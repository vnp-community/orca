package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// PairedDeviceStore implements usecase.PairedDeviceRepository against
// paired_devices — mirrors internal/adapter/postgres.PairedDeviceStore's
// own-struct shape.
type PairedDeviceStore struct {
	db *sql.DB
}

func NewPairedDeviceStore(db *sql.DB) *PairedDeviceStore {
	return &PairedDeviceStore{db: db}
}

func (s *PairedDeviceStore) Save(ctx context.Context, device domain.PairedDevice) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO paired_devices
			(id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		device.ID, device.TenantID, device.UserID, device.DeviceLabel, device.SharedSecretCiphertext,
		device.VaultKeyRef, string(device.Status), device.PairedAt, nullTime(device.LastUsedAt), device.RevokedAt)
	if err != nil {
		return fmt.Errorf("mysql: insert paired device: %w", err)
	}
	return nil
}

// CountActive returns the number of currently-active paired devices for
// (tenantID, userID) — backs BR-MB-03's cap check.
func (s *PairedDeviceStore) CountActive(ctx context.Context, tenantID, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM paired_devices
		WHERE tenant_id = ? AND user_id = ? AND status = 'active'`,
		tenantID, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("mysql: count active paired devices: %w", err)
	}
	return n, nil
}

func (s *PairedDeviceStore) Get(ctx context.Context, id string) (domain.PairedDevice, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at
		FROM paired_devices
		WHERE id = ?`, id)
	device, err := scanPairedDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PairedDevice{}, fmt.Errorf("mysql: get paired device: %w", domain.ErrDeviceNotFound)
	}
	if err != nil {
		return domain.PairedDevice{}, fmt.Errorf("mysql: get paired device: %w", err)
	}
	return device, nil
}

func (s *PairedDeviceStore) List(ctx context.Context, tenantID, userID string) ([]domain.PairedDevice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at
		FROM paired_devices
		WHERE tenant_id = ? AND user_id = ?
		ORDER BY paired_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list paired devices: %w", err)
	}
	defer rows.Close()

	var out []domain.PairedDevice
	for rows.Next() {
		device, err := scanPairedDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan paired device row: %w", err)
		}
		out = append(out, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate paired device rows: %w", err)
	}
	return out, nil
}

// RevokeAndWipeSecret marks a device revoked AND nulls its shared-secret
// ciphertext/key ref in the same statement — BR-MB-04's enforcement point.
// Does NOT branch on UPDATE's RowsAffected()==0 to detect "not found" —
// same class of bug as RevokeSession/Revoke elsewhere in this package: a
// retried revoke of an already-revoked, already-wiped device (every SET
// value unchanged from the prior call) would hit RowsAffected()==0 under
// MySQL's default "rows changed" semantics and incorrectly report
// ErrDeviceNotFound for a device that DOES exist — this is BR-MB-04's own
// secret-wipe guarantee, a security-relevant path, so a false not-found
// here must not be allowed to mask "this device's secret still needs
// wiping". Instead: UPDATE unconditionally, then confirm existence with a
// follow-up SELECT.
func (s *PairedDeviceStore) RevokeAndWipeSecret(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE paired_devices
		SET status = 'revoked', shared_secret_ciphertext = NULL, vault_key_ref = NULL, revoked_at = NOW(6)
		WHERE id = ?`, id); err != nil {
		return fmt.Errorf("mysql: revoke paired device: %w", err)
	}

	var exists string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM paired_devices WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: revoke paired device: %w", domain.ErrDeviceNotFound)
	}
	if err != nil {
		return fmt.Errorf("mysql: revoke paired device: confirming device exists: %w", err)
	}
	return nil
}

// Touch updates last_used_at — best-effort, no not-found detection in
// either dialect.
func (s *PairedDeviceStore) Touch(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE paired_devices SET last_used_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("mysql: touch paired device: %w", err)
	}
	return nil
}

func scanPairedDevice(row rowScanner) (domain.PairedDevice, error) {
	var d domain.PairedDevice
	var status string
	var deviceLabel *string
	var secretCiphertext []byte
	var vaultKeyRef *string
	var lastUsedAt *time.Time
	err := row.Scan(&d.ID, &d.TenantID, &d.UserID, &deviceLabel, &secretCiphertext, &vaultKeyRef, &status, &d.PairedAt, &lastUsedAt, &d.RevokedAt)
	if err != nil {
		return domain.PairedDevice{}, err
	}
	if deviceLabel != nil {
		d.DeviceLabel = *deviceLabel
	}
	if vaultKeyRef != nil {
		d.VaultKeyRef = *vaultKeyRef
	}
	if lastUsedAt != nil {
		d.LastUsedAt = *lastUsedAt
	}
	d.SharedSecretCiphertext = secretCiphertext
	d.Status = domain.DeviceStatus(status)
	return d, nil
}

// nullTime returns nil for a zero time.Time so an unset LastUsedAt is
// stored as SQL NULL.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
