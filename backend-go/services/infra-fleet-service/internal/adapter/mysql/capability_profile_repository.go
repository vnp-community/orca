package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CapabilityProfileStore persists dev_server_capability_profiles. MySQL has
// no RLS, so the tenant_id filter in every statement is the only guard.
type CapabilityProfileStore struct {
	db *sql.DB
}

func NewCapabilityProfileStore(db *sql.DB) *CapabilityProfileStore {
	return &CapabilityProfileStore{db: db}
}

func (s *CapabilityProfileStore) Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	var (
		p                          domain.CapabilityProfile
		featuresJSON, profileBytes []byte
		source                     string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
		FROM dev_server_capability_profiles
		WHERE tenant_id = ? AND dev_server_id = ?
	`, tenantID, devServerID).Scan(&source, &p.AgentBuildVersion, &p.ProtocolVersion, &featuresJSON, &profileBytes, &p.Fingerprint, &p.ProbedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CapabilityProfile{}, false, nil
	}
	if err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("mysql: get capability profile: %w", err)
	}
	p.DevServerID = devServerID
	p.TenantID = tenantID
	p.Source = domain.ProfileSource(source)
	p.ProbedAt = p.ProbedAt.UTC()
	p.ProfileJSON = profileBytes
	if err := json.Unmarshal(featuresJSON, &p.Features); err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("mysql: decode capability features: %w", err)
	}
	return p, true, nil
}

// Upsert locks the owning dev_servers row first so two concurrent probes are
// serialized and exactly one sees existed=true; a tenant mismatch or missing
// dev server returns domain.ErrNotFound.
func (s *CapabilityProfileStore) Upsert(ctx context.Context, p domain.CapabilityProfile) (string, bool, error) {
	if len(p.ProfileJSON) > domain.MaxProfileJSONBytes {
		return "", false, domain.ErrProfileTooLarge
	}
	featuresJSON, err := json.Marshal(domain.NormalizeFeatures(p.Features))
	if err != nil {
		return "", false, fmt.Errorf("mysql: encode capability features: %w", err)
	}
	profileJSON := p.ProfileJSON
	if len(profileJSON) == 0 {
		profileJSON = []byte("{}")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM dev_servers WHERE id = ? AND tenant_id = ? FOR UPDATE`,
		p.DevServerID, p.TenantID).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return "", false, domain.ErrNotFound
	} else if err != nil {
		return "", false, fmt.Errorf("mysql: lock dev server: %w", err)
	}

	var prev string
	var existed bool
	switch err := tx.QueryRowContext(ctx, `
		SELECT fingerprint FROM dev_server_capability_profiles WHERE tenant_id = ? AND dev_server_id = ?
	`, p.TenantID, p.DevServerID).Scan(&prev); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return "", false, fmt.Errorf("mysql: read previous fingerprint: %w", err)
	default:
		existed = true
	}

	// JSON columns reject []byte args (binary charset); pass strings.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO dev_server_capability_profiles
			(dev_server_id, tenant_id, source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			source = VALUES(source),
			agent_build_version = VALUES(agent_build_version),
			protocol_version = VALUES(protocol_version),
			features = VALUES(features),
			profile = VALUES(profile),
			fingerprint = VALUES(fingerprint),
			probed_at = VALUES(probed_at),
			updated_at = CURRENT_TIMESTAMP(6)
	`, p.DevServerID, p.TenantID, string(p.Source), p.AgentBuildVersion, p.ProtocolVersion,
		string(featuresJSON), string(profileJSON), p.Fingerprint, p.ProbedAt.UTC().Truncate(time.Microsecond)); err != nil {
		return "", false, fmt.Errorf("mysql: upsert capability profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("mysql: commit: %w", err)
	}
	return prev, existed, nil
}

// TenantIDForDevServer implements usecase.DevServerTenantLookup.
func (s *CapabilityProfileStore) TenantIDForDevServer(ctx context.Context, devServerID string) (string, bool, error) {
	var tenantID string
	err := s.db.QueryRowContext(ctx, `SELECT tenant_id FROM dev_servers WHERE id = ?`, devServerID).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("mysql: lookup dev server tenant: %w", err)
	}
	return tenantID, true, nil
}
