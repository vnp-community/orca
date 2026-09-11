package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerAccessRequestStore implements
// usecase.DevServerAccessRequestRepository against
// dev_server_access_requests (migrations/mysql/0033).
type DevServerAccessRequestStore struct {
	db *sql.DB
}

func NewDevServerAccessRequestStore(db *sql.DB) *DevServerAccessRequestStore {
	return &DevServerAccessRequestStore{db: db}
}

const accessRequestColumns = `id, tenant_id, user_id, dev_server_group_id, status, message, grantee_kind, grantee_id, created_at`

func (s *DevServerAccessRequestStore) Create(ctx context.Context, req domain.DevServerAccessRequest) (domain.DevServerAccessRequest, error) {
	createdAt := time.UnixMilli(req.CreatedAtUnixMs)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dev_server_access_requests (id, tenant_id, user_id, dev_server_group_id, status, message, grantee_kind, grantee_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.ID, req.TenantID, req.UserID, req.DevServerGroupID, string(req.Status), req.Message, string(req.GranteeKind), req.GranteeID, createdAt)
	if err != nil {
		return domain.DevServerAccessRequest{}, fmt.Errorf("mysql: insert dev server access request: %w", err)
	}
	return req, nil
}

func (s *DevServerAccessRequestStore) Get(ctx context.Context, tenantID, id string) (domain.DevServerAccessRequest, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+accessRequestColumns+`
		FROM dev_server_access_requests
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	return scanAccessRequestRow(row)
}

func (s *DevServerAccessRequestStore) ListPending(ctx context.Context, tenantID string) ([]domain.DevServerAccessRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+accessRequestColumns+`
		FROM dev_server_access_requests
		WHERE tenant_id = ? AND status = 'pending'
		ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query pending access requests: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerAccessRequest
	for rows.Next() {
		req, err := scanAccessRequestRow(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan access request row: %w", err)
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate access request rows: %w", err)
	}
	return out, nil
}

// UpdateStatus has no RETURNING on MySQL — UPDATE then SELECT back by
// (tenant_id, id), per BE-DB-SOL-005 §3.1's translation (this column has no
// RowsAffected-vs-RETURNING ambiguity to worry about either way, since
// not-found is decided by the follow-up SELECT, never by RowsAffected()).
func (s *DevServerAccessRequestStore) UpdateStatus(ctx context.Context, tenantID, id string, status domain.AccessRequestStatus) (domain.DevServerAccessRequest, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE dev_server_access_requests
		SET status = ?, resolved_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND id = ?
	`, string(status), tenantID, id)
	if err != nil {
		return domain.DevServerAccessRequest{}, fmt.Errorf("mysql: update dev server access request status: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+accessRequestColumns+`
		FROM dev_server_access_requests
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	return scanAccessRequestRow(row)
}

// scanAccessRequestRow uses the package-level rowScanner interface
// (shared.go) — satisfied by both *sql.Row and *sql.Rows — to serve
// Get/UpdateStatus (single row) and ListPending (iterated rows) alike.
func scanAccessRequestRow(row rowScanner) (domain.DevServerAccessRequest, error) {
	var req domain.DevServerAccessRequest
	var status, kind string
	var createdAt time.Time
	err := row.Scan(&req.ID, &req.TenantID, &req.UserID, &req.DevServerGroupID, &status, &req.Message, &kind, &req.GranteeID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServerAccessRequest{}, fmt.Errorf("mysql: access request not found for tenant: %w", err)
	}
	if err != nil {
		return domain.DevServerAccessRequest{}, fmt.Errorf("mysql: scan access request: %w", err)
	}
	req.Status = domain.AccessRequestStatus(status)
	req.GranteeKind = domain.GranteeKind(kind)
	req.CreatedAtUnixMs = createdAt.UnixMilli()
	return req, nil
}
