package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type CapabilityProfileStore struct {
	db *sql.DB
}

func NewCapabilityProfileStore(db *sql.DB) *CapabilityProfileStore {
	return &CapabilityProfileStore{db: db}
}

func (r *CapabilityProfileStore) Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	query := `
		SELECT source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
		FROM dev_server_capability_profiles
		WHERE dev_server_id = ? AND tenant_id = ?
	`
	var p domain.CapabilityProfile
	p.DevServerID = devServerID
	p.TenantID = tenantID

	var featuresJSON, profileJSON []byte
	err := r.db.QueryRowContext(ctx, query, devServerID, tenantID).Scan(
		&p.Source,
		&p.AgentBuildVersion,
		&p.ProtocolVersion,
		&featuresJSON,
		&profileJSON,
		&p.Fingerprint,
		&p.ProbedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CapabilityProfile{}, false, nil
	}
	if err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("mysql: get capability profile: %w", err)
	}

	if err := json.Unmarshal(featuresJSON, &p.Features); err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("mysql: decode features: %w", err)
	}
	p.ProfileJSON = profileJSON

	return p, true, nil
}

func (r *CapabilityProfileStore) Upsert(ctx context.Context, p domain.CapabilityProfile) (string, bool, error) {
	if len(p.ProfileJSON) > 65535 {
		return "", false, domain.ErrProfileTooLarge
	}

	featuresJSON, err := json.Marshal(p.Features)
	if err != nil {
		return "", false, fmt.Errorf("encode features: %w", err)
	}
	profileJSON := p.ProfileJSON
	if len(profileJSON) == 0 {
		profileJSON = []byte("{}")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var prevFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT fingerprint FROM dev_server_capability_profiles
		WHERE dev_server_id = ? AND tenant_id = ?
		FOR UPDATE
	`, p.DevServerID, p.TenantID).Scan(&prevFingerprint)

	var existed bool
	if errors.Is(err, sql.ErrNoRows) {
		// Does not exist yet.
	} else if err != nil {
		return "", false, fmt.Errorf("select for update: %w", err)
	} else {
		existed = true
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO dev_server_capability_profiles (
			dev_server_id, tenant_id, source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?
		FROM dev_servers
		WHERE id = ? AND tenant_id = ?
		ON DUPLICATE KEY UPDATE
			source = VALUES(source),
			agent_build_version = VALUES(agent_build_version),
			protocol_version = VALUES(protocol_version),
			features = VALUES(features),
			profile = VALUES(profile),
			fingerprint = VALUES(fingerprint),
			probed_at = VALUES(probed_at),
			updated_at = CURRENT_TIMESTAMP(6)
	`, p.DevServerID, p.TenantID, p.Source, p.AgentBuildVersion, p.ProtocolVersion, featuresJSON, profileJSON, p.Fingerprint, p.ProbedAt,
		p.DevServerID, p.TenantID)

	if err != nil {
		return "", false, fmt.Errorf("mysql: upsert capability profile: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 && !existed {
		return "", false, domain.ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit tx: %w", err)
	}

	return prevFingerprint, existed, nil
}
