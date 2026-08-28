package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// PairedDeviceStore implements usecase.PairedDeviceRepository against
// auth.paired_devices.
type PairedDeviceStore struct {
	pool *pgxpool.Pool
}

func NewPairedDeviceStore(pool *pgxpool.Pool) *PairedDeviceStore {
	return &PairedDeviceStore{pool: pool}
}

func (s *PairedDeviceStore) Save(ctx context.Context, device domain.PairedDevice) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.paired_devices
			(id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		device.ID, device.TenantID, device.UserID, device.DeviceLabel, device.SharedSecretCiphertext,
		device.VaultKeyRef, string(device.Status), device.PairedAt, nullTime(device.LastUsedAt), device.RevokedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert paired device: %w", err)
	}
	return nil
}

// CountActive returns the number of currently-active paired devices for
// (tenantID, userID) — backs BR-MB-03's cap check.
func (s *PairedDeviceStore) CountActive(ctx context.Context, tenantID, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM auth.paired_devices
		WHERE tenant_id = $1 AND user_id = $2 AND status = 'active'`,
		tenantID, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("postgres: count active paired devices: %w", err)
	}
	return n, nil
}

func (s *PairedDeviceStore) Get(ctx context.Context, id string) (domain.PairedDevice, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at
		FROM auth.paired_devices
		WHERE id = $1`, id)
	device, err := scanPairedDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PairedDevice{}, fmt.Errorf("postgres: get paired device: %w", domain.ErrDeviceNotFound)
	}
	if err != nil {
		return domain.PairedDevice{}, fmt.Errorf("postgres: get paired device: %w", err)
	}
	return device, nil
}

func (s *PairedDeviceStore) List(ctx context.Context, tenantID, userID string) ([]domain.PairedDevice, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, device_label, shared_secret_ciphertext, vault_key_ref, status, paired_at, last_used_at, revoked_at
		FROM auth.paired_devices
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY paired_at DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list paired devices: %w", err)
	}
	defer rows.Close()

	var out []domain.PairedDevice
	for rows.Next() {
		device, err := scanPairedDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan paired device row: %w", err)
		}
		out = append(out, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate paired device rows: %w", err)
	}
	return out, nil
}

// RevokeAndWipeSecret marks a device revoked AND nulls its shared-secret
// ciphertext/key ref in the same statement — BR-MB-04's enforcement point:
// ResolveDeviceSharedSecret can never again return this device's secret,
// not even from a stale replica, because the plaintext-recoverable material
// is gone, not just flagged.
func (s *PairedDeviceStore) RevokeAndWipeSecret(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE auth.paired_devices
		SET status = 'revoked', shared_secret_ciphertext = NULL, vault_key_ref = NULL, revoked_at = now()
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: revoke paired device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: revoke paired device: %w", domain.ErrDeviceNotFound)
	}
	return nil
}

// Touch updates last_used_at — called (best-effort) whenever
// ResolveDeviceSharedSecret successfully decrypts a device's secret.
func (s *PairedDeviceStore) Touch(ctx context.Context, id string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth.paired_devices SET last_used_at = $2 WHERE id = $1`, id, now)
	if err != nil {
		return fmt.Errorf("postgres: touch paired device: %w", err)
	}
	return nil
}

// scanPairedDevice uses the shared rowScanner interface (user_repository.go)
// so it serves both QueryRow and Query call sites.
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
// stored as SQL NULL rather than Postgres's epoch-zero timestamp.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
