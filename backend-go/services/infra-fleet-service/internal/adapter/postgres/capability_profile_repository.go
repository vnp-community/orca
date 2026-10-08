package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var (
	_ usecase.CapabilityProfileStore = (*CapabilityProfileStore)(nil)
	_ usecase.DevServerTenantLookup  = (*CapabilityProfileStore)(nil)
	_ usecase.OutboxEnqueuer         = (*Repository)(nil)
)

// CapabilityProfileStore persists infra.dev_server_capability_profiles.
// The table has FORCE ROW LEVEL SECURITY, so every query runs in a
// transaction that sets app.tenant_id; the WHERE tenant_id filters remain as
// the primary guard.
type CapabilityProfileStore struct {
	pool *pgxpool.Pool
}

func NewCapabilityProfileStore(pool *pgxpool.Pool) *CapabilityProfileStore {
	return &CapabilityProfileStore{pool: pool}
}

func (s *CapabilityProfileStore) inTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("postgres: set tenant: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

func (s *CapabilityProfileStore) Get(ctx context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	var (
		p                          domain.CapabilityProfile
		found                      bool
		featuresJSON, profileBytes []byte
		source, fingerprint        string
	)
	err := s.inTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at
			FROM infra.dev_server_capability_profiles
			WHERE tenant_id = $1 AND dev_server_id = $2
		`, tenantID, devServerID).Scan(&source, &p.AgentBuildVersion, &p.ProtocolVersion, &featuresJSON, &profileBytes, &fingerprint, &p.ProbedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get capability profile: %w", err)
		}
		found = true
		return nil
	})
	if err != nil || !found {
		return domain.CapabilityProfile{}, false, err
	}
	p.DevServerID = devServerID
	p.TenantID = tenantID
	p.Source = domain.ProfileSource(source)
	p.Fingerprint = strings.TrimRight(fingerprint, " ")
	p.ProbedAt = p.ProbedAt.UTC()
	p.ProfileJSON = profileBytes
	if err := json.Unmarshal(featuresJSON, &p.Features); err != nil {
		return domain.CapabilityProfile{}, false, fmt.Errorf("postgres: decode capability features: %w", err)
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
		return "", false, fmt.Errorf("postgres: encode capability features: %w", err)
	}
	profileJSON := p.ProfileJSON
	if len(profileJSON) == 0 {
		profileJSON = []byte("{}")
	}

	var prev string
	var existed bool
	err = s.inTenantTx(ctx, p.TenantID, func(tx pgx.Tx) error {
		var one int
		if err := tx.QueryRow(ctx, `
			SELECT 1 FROM infra.dev_servers WHERE id = $1 AND tenant_id = $2 FOR NO KEY UPDATE
		`, p.DevServerID, p.TenantID).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("postgres: lock dev server: %w", err)
		}

		var prevFP string
		switch err := tx.QueryRow(ctx, `
			SELECT fingerprint FROM infra.dev_server_capability_profiles
			WHERE tenant_id = $1 AND dev_server_id = $2
		`, p.TenantID, p.DevServerID).Scan(&prevFP); {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("postgres: read previous fingerprint: %w", err)
		default:
			existed = true
			prev = strings.TrimRight(prevFP, " ")
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO infra.dev_server_capability_profiles
				(dev_server_id, tenant_id, source, agent_build_version, protocol_version, features, profile, fingerprint, probed_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)
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
		`, p.DevServerID, p.TenantID, string(p.Source), p.AgentBuildVersion, p.ProtocolVersion,
			string(featuresJSON), string(profileJSON), p.Fingerprint, p.ProbedAt.UTC().Truncate(time.Microsecond)); err != nil {
			return fmt.Errorf("postgres: upsert capability profile: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return prev, existed, nil
}

// TenantIDForDevServer implements usecase.DevServerTenantLookup.
func (s *CapabilityProfileStore) TenantIDForDevServer(ctx context.Context, devServerID string) (string, bool, error) {
	var tenantID string
	err := s.pool.QueryRow(ctx, `SELECT tenant_id::text FROM infra.dev_servers WHERE id = $1`, devServerID).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("postgres: lookup dev server tenant: %w", err)
	}
	return tenantID, true, nil
}
