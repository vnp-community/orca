package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type CapabilityProfileStore struct {
	pool *pgxpool.Pool
}

func NewCapabilityProfileStore(pool *pgxpool.Pool) *CapabilityProfileStore {
	return &CapabilityProfileStore{pool: pool}
}

func (r *CapabilityProfileStore) Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	query := `
		SELECT source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
		FROM infra.dev_server_capability_profiles
		WHERE dev_server_id = $1 AND tenant_id = $2
	`
	var p domain.CapabilityProfile
	p.DevServerID = devServerID
	p.TenantID = tenantID

	var featuresJSON, profileJSON []byte
	err := r.pool.QueryRow(ctx, query, devServerID, tenantID).Scan(
		&p.Source,
		&p.AgentBuildVersion,
		&p.ProtocolVersion,
		&featuresJSON,
		&profileJSON,
		&p.Fingerprint,
		&p.ProbedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CapabilityProfile{}, false, nil
	}
	if err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("postgres: get capability profile: %w", err)
	}

	if err := json.Unmarshal(featuresJSON, &p.Features); err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("postgres: decode features: %w", err)
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

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var prevFingerprint string
	err = tx.QueryRow(ctx, `
		SELECT fingerprint FROM infra.dev_server_capability_profiles
		WHERE dev_server_id = $1 AND tenant_id = $2
		FOR UPDATE
	`, p.DevServerID, p.TenantID).Scan(&prevFingerprint)

	var existed bool
	if errors.Is(err, pgx.ErrNoRows) {
		// Needs to make sure tenantID matches the dev_server's tenantID
		// Since we do INSERT ON CONFLICT, if dev_server doesn't exist it fails FK.
		// If it exists but wrong tenant, it won't be caught by the INSERT ON CONFLICT until maybe a trigger,
		// wait, ON CONFLICT DO UPDATE WHERE tenant_id = $2. If tenant differs, the UPDATE is skipped?
		// But let's check dev_server's tenant first to be safe, or let the INSERT happen.
		// If we insert with wrong tenant, and dev_server belongs to another tenant, should we prevent it?
		// We could verify dev_server's tenant.
		// Spec says "chỉ cập nhật khi tenant_id khớp (WHERE ... .tenant_id = EXCLUDED.tenant_id), tenant khác trả lỗi domain.ErrNotFound".
		// Actually, if we do:
		// INSERT INTO ... ON CONFLICT (dev_server_id) DO UPDATE SET ... WHERE dev_server_capability_profiles.tenant_id = EXCLUDED.tenant_id
		// If tenant doesn't match, the UPDATE is skipped and INSERT is skipped, so nothing happens.
	} else if err != nil {
		return "", false, fmt.Errorf("select for update: %w", err)
	} else {
		existed = true
	}

	// But wait, if existed is true, and we UPDATE, it's fine.
	// If it doesn't exist, how to know if dev_server exists and tenant matches?
	// The problem is ON CONFLICT DO UPDATE will insert if not exists. If dev_server belongs to tenant A, and we insert for tenant B,
	// Postgres foreign key doesn't check tenant_id (only dev_server_id).
	// So we might need to check if dev_server belongs to tenant B before inserting.
	// We can do:
	// INSERT INTO ... (dev_server_id, tenant_id, ...)
	// SELECT id, tenant_id, ... FROM infra.dev_servers WHERE id = $1 AND tenant_id = $2
	// ON CONFLICT (dev_server_id) DO UPDATE SET ... WHERE dev_server_capability_profiles.tenant_id = EXCLUDED.tenant_id

	res, err := tx.Exec(ctx, `
		INSERT INTO infra.dev_server_capability_profiles (
			dev_server_id, tenant_id, source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
		FROM infra.dev_servers
		WHERE id = $1 AND tenant_id = $2
		ON CONFLICT (dev_server_id) DO UPDATE SET
			source = EXCLUDED.source,
			agent_build_version = EXCLUDED.agent_build_version,
			protocol_version = EXCLUDED.protocol_version,
			features = EXCLUDED.features,
			profile = EXCLUDED.profile,
			fingerprint = EXCLUDED.fingerprint,
			probed_at = EXCLUDED.probed_at,
			updated_at = now()
		WHERE infra.dev_server_capability_profiles.tenant_id = EXCLUDED.tenant_id
	`, p.DevServerID, p.TenantID, p.Source, p.AgentBuildVersion, p.ProtocolVersion, featuresJSON, profileJSON, p.Fingerprint, p.ProbedAt)

	if err != nil {
		return "", false, fmt.Errorf("postgres: upsert capability profile: %w", err)
	}

	if res.RowsAffected() == 0 {
		return "", false, domain.ErrNotFound // Could mean dev_server not found or tenant mismatched
	}

	if err := tx.Commit(ctx); err != nil {
		return "", false, fmt.Errorf("commit tx: %w", err)
	}

	return prevFingerprint, existed, nil
}
