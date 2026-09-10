package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerAccessRequestStore implements
// usecase.DevServerAccessRequestRepository against
// infra.dev_server_access_requests
// (migrations/0009_dev_server_group_grants_and_access_requests) — see
// docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md.
type DevServerAccessRequestStore struct {
	pool *pgxpool.Pool
}

func NewDevServerAccessRequestStore(pool *pgxpool.Pool) *DevServerAccessRequestStore {
	return &DevServerAccessRequestStore{pool: pool}
}

const accessRequestColumns = `id, tenant_id, user_id, dev_server_group_id, status, message, grantee_kind, grantee_id, created_at`

func (s *DevServerAccessRequestStore) Create(ctx context.Context, req domain.DevServerAccessRequest) (domain.DevServerAccessRequest, error) {
	createdAt := time.UnixMilli(req.CreatedAtUnixMs)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.dev_server_access_requests (id, tenant_id, user_id, dev_server_group_id, status, message, grantee_kind, grantee_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, req.ID, req.TenantID, req.UserID, req.DevServerGroupID, string(req.Status), req.Message, string(req.GranteeKind), req.GranteeID, createdAt)
	if err != nil {
		return domain.DevServerAccessRequest{}, fmt.Errorf("postgres: insert dev server access request: %w", err)
	}
	return req, nil
}

func (s *DevServerAccessRequestStore) Get(ctx context.Context, tenantID, id string) (domain.DevServerAccessRequest, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+accessRequestColumns+`
		FROM infra.dev_server_access_requests
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	return scanAccessRequestRow(row)
}

func (s *DevServerAccessRequestStore) ListPending(ctx context.Context, tenantID string) ([]domain.DevServerAccessRequest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+accessRequestColumns+`
		FROM infra.dev_server_access_requests
		WHERE tenant_id = $1 AND status = 'pending'
		ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query pending access requests: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerAccessRequest
	for rows.Next() {
		req, err := scanAccessRequestRow(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan access request row: %w", err)
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate access request rows: %w", err)
	}
	return out, nil
}

func (s *DevServerAccessRequestStore) UpdateStatus(ctx context.Context, tenantID, id string, status domain.AccessRequestStatus) (domain.DevServerAccessRequest, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE infra.dev_server_access_requests
		SET status = $3, resolved_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+accessRequestColumns, tenantID, id, string(status))
	return scanAccessRequestRow(row)
}

// scanAccessRequestRow uses the package-level rowScanner interface
// (terminal_session_repository.go) — satisfied by both pgx.Row and
// pgx.Rows — to serve Get/UpdateStatus (single row) and ListPending
// (iterated rows) alike.
func scanAccessRequestRow(row rowScanner) (domain.DevServerAccessRequest, error) {
	var req domain.DevServerAccessRequest
	var status, kind string
	var createdAt time.Time
	err := row.Scan(&req.ID, &req.TenantID, &req.UserID, &req.DevServerGroupID, &status, &req.Message, &kind, &req.GranteeID, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServerAccessRequest{}, fmt.Errorf("postgres: access request not found for tenant: %w", err)
	}
	if err != nil {
		return domain.DevServerAccessRequest{}, fmt.Errorf("postgres: scan access request: %w", err)
	}
	req.Status = domain.AccessRequestStatus(status)
	req.GranteeKind = domain.GranteeKind(kind)
	req.CreatedAtUnixMs = createdAt.UnixMilli()
	return req, nil
}
