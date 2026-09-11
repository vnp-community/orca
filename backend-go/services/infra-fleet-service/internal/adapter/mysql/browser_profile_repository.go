package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var _ usecase.BrowserProfileRepository = (*BrowserProfileStore)(nil)

// BrowserProfileStore implements usecase.BrowserProfileRepository against
// browser_profiles (migrations/mysql/0006).
type BrowserProfileStore struct {
	db *sql.DB
}

func NewBrowserProfileStore(db *sql.DB) *BrowserProfileStore {
	return &BrowserProfileStore{db: db}
}

func (s *BrowserProfileStore) List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error) {
	const q = `
		SELECT id, tenant_id, dev_server_id, name, source_browser, is_default, created_at
		FROM browser_profiles
		WHERE tenant_id = ? AND dev_server_id = ?
		ORDER BY created_at`
	rows, err := s.db.QueryContext(ctx, q, tenantID, devServerID)
	if err != nil {
		return nil, fmt.Errorf("mysql: listing browser profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.BrowserProfile
	for rows.Next() {
		var p domain.BrowserProfile
		var sourceBrowser sql.NullString
		if err := rows.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &sourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scanning browser profile: %w", err)
		}
		if sourceBrowser.Valid {
			p.SourceBrowser = sourceBrowser.String
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// Create inserts new browser profile metadata and returns the persisted
// row. No RETURNING on MySQL — INSERT then SELECT back by primary key,
// mirroring BE-DB-SOL-005 §3's UPDATE-then-SELECT translation (this is the
// INSERT-side equivalent: every column of the freshly-inserted row is
// already known from profile itself, except source_browser's NULL
// normalization, which COALESCE handles the same way the Postgres RETURNING
// clause did).
func (s *BrowserProfileStore) Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO browser_profiles (id, tenant_id, dev_server_id, name, source_browser, is_default)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), ?)
	`, profile.ID, profile.TenantID, profile.DevServerID, profile.Name, profile.SourceBrowser, profile.IsDefault)
	if err != nil {
		return domain.BrowserProfile{}, fmt.Errorf("mysql: creating browser profile: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, dev_server_id, name, COALESCE(source_browser, ''), is_default, created_at
		FROM browser_profiles WHERE id = ?`, profile.ID)
	var p domain.BrowserProfile
	if err := row.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &p.SourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
		return domain.BrowserProfile{}, fmt.Errorf("mysql: reading back created browser profile: %w", err)
	}
	return p, nil
}

// Delete removes browser profile metadata scoped to tenantID. DELETE's
// RowsAffected() counts by WHERE-match on every dialect (see
// dev_server_group_grant_repository.go's Delete comment) — no
// UPDATE-vs-RowsAffected pitfall here.
func (s *BrowserProfileStore) Delete(ctx context.Context, tenantID, id string) error {
	const q = `DELETE FROM browser_profiles WHERE tenant_id = ? AND id = ?`
	res, err := s.db.ExecContext(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: deleting browser profile: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mysql: browser profile %s not found", id)
	}
	return nil
}
