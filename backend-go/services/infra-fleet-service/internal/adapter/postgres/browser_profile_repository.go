package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var _ usecase.BrowserProfileRepository = (*BrowserProfileStore)(nil)

// BrowserProfileStore implements usecase.BrowserProfileRepository against
// infra.browser_profiles (migrations/0004_browser_profiles) — split into its
// own type over the same pool Repository uses, rather than methods on
// Repository directly, because Repository.List/Create already exist with
// different signatures for DevServerRepository/ConnectionRepository (same
// method-name-collision reasoning as SshTargetStore, see Repository's doc
// comment).
type BrowserProfileStore struct {
	pool *pgxpool.Pool
}

// NewBrowserProfileStore builds a BrowserProfileStore over the same pool
// Repository/SshTargetStore use — see Repository's doc comment for why this
// isn't the same Go value as Repository.
func NewBrowserProfileStore(pool *pgxpool.Pool) *BrowserProfileStore {
	return &BrowserProfileStore{pool: pool}
}

// List returns the browser profiles registered for devServerID, scoped to
// tenantID — see migrations/0004_browser_profiles.up.sql.
func (s *BrowserProfileStore) List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error) {
	const q = `
		SELECT id, tenant_id, dev_server_id, name, source_browser, is_default, created_at
		FROM infra.browser_profiles
		WHERE tenant_id = $1 AND dev_server_id = $2
		ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, tenantID, devServerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing browser profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.BrowserProfile
	for rows.Next() {
		var p domain.BrowserProfile
		var sourceBrowser *string
		if err := rows.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &sourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning browser profile: %w", err)
		}
		if sourceBrowser != nil {
			p.SourceBrowser = *sourceBrowser
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// Create inserts new browser profile metadata and returns the persisted row.
func (s *BrowserProfileStore) Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error) {
	const q = `
		INSERT INTO infra.browser_profiles (id, tenant_id, dev_server_id, name, source_browser, is_default)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		RETURNING id, tenant_id, dev_server_id, name, COALESCE(source_browser, ''), is_default, created_at`
	row := s.pool.QueryRow(ctx, q, profile.ID, profile.TenantID, profile.DevServerID, profile.Name, profile.SourceBrowser, profile.IsDefault)
	var p domain.BrowserProfile
	if err := row.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &p.SourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
		return domain.BrowserProfile{}, fmt.Errorf("postgres: creating browser profile: %w", err)
	}
	return p, nil
}

// Delete removes browser profile metadata scoped to tenantID.
func (s *BrowserProfileStore) Delete(ctx context.Context, tenantID, id string) error {
	const q = `DELETE FROM infra.browser_profiles WHERE tenant_id = $1 AND id = $2`
	tag, err := s.pool.Exec(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: deleting browser profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: browser profile %s not found", id)
	}
	return nil
}
