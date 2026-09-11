package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

var _ usecase.ConnectionRepository = (*Repository)(nil)

// Upsert mirrors internal/adapter/postgres/connections.go's Upsert
// semantics exactly, but MySQL's ON DUPLICATE KEY UPDATE can't express
// Postgres's `NOT EXISTS (...)` inline VALUES expression for is_selected's
// insert-only default — that scalar subquery reads the very table being
// inserted into, which is uncertain/version-fragile MySQL/TiDB syntax to
// rely on. Instead this does the existence check as its own SELECT inside
// the same transaction, then passes the computed bool as a plain
// parameter — same net effect (first connection for
// (tenant,user,provider) is auto-selected; a later additional workspace
// defaults unselected), just computed in Go instead of SQL.
func (r *Repository) Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: begin upsert tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM connections WHERE tenant_id=? AND user_id=? AND provider=?`,
		tenantID, userID, string(provider)).Scan(&existing); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: check existing connections: %w", err)
	}
	isFirstConnection := existing == 0

	_, err = tx.ExecContext(ctx, `
		INSERT INTO connections
			(tenant_id, user_id, provider, external_workspace_id, workspace_name, workspace_url,
			 viewer_id, viewer_display_name, viewer_email, credential_id, is_selected, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			workspace_name = VALUES(workspace_name), workspace_url = VALUES(workspace_url),
			viewer_id = VALUES(viewer_id), viewer_display_name = VALUES(viewer_display_name),
			viewer_email = VALUES(viewer_email), credential_id = VALUES(credential_id), updated_at = NOW(6)
	`, tenantID, userID, string(provider), workspace.ID, workspace.Name, workspace.URL,
		viewer.ID, viewer.DisplayName, viewer.Email, credentialID, isFirstConnection)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: upsert connection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: commit upsert tx: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error {
	if workspaceID == "" {
		_, err := r.db.ExecContext(ctx, `DELETE FROM connections WHERE tenant_id=? AND user_id=? AND provider=?`, tenantID, userID, string(provider))
		if err != nil {
			return fmt.Errorf("mysql: delete all connections: %w", err)
		}
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM connections WHERE tenant_id=? AND user_id=? AND provider=? AND external_workspace_id=?`,
		tenantID, userID, string(provider), workspaceID)
	if err != nil {
		return fmt.Errorf("mysql: delete connection: %w", err)
	}
	return nil
}

func (r *Repository) GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_workspace_id, workspace_name, workspace_url,
		       viewer_id, viewer_display_name, viewer_email, is_selected
		FROM connections
		WHERE tenant_id=? AND user_id=? AND provider=?
		ORDER BY created_at
	`, tenantID, userID, string(provider))
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: query connection status: %w", err)
	}
	defer rows.Close()

	var status domain.ConnectionStatus
	for rows.Next() {
		var ws domain.Workspace
		var viewer domain.Viewer
		var selected bool
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.URL, &viewer.ID, &viewer.DisplayName, &viewer.Email, &selected); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("mysql: scan connection row: %w", err)
		}
		status.Workspaces = append(status.Workspaces, ws)
		if selected {
			status.SelectedWorkspaceID = ws.ID
			status.ActiveWorkspaceID = ws.ID
			status.Viewer = viewer
		}
	}
	if err := rows.Err(); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: iterate connection rows: %w", err)
	}
	status.Connected = len(status.Workspaces) > 0
	return status, nil
}

func (r *Repository) SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: begin select-workspace tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE connections SET is_selected=false WHERE tenant_id=? AND user_id=? AND provider=?`,
		tenantID, userID, string(provider)); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: clear selection: %w", err)
	}
	if workspaceID != "" && workspaceID != "all" {
		if _, err := tx.ExecContext(ctx, `UPDATE connections SET is_selected=true WHERE tenant_id=? AND user_id=? AND provider=? AND external_workspace_id=?`,
			tenantID, userID, string(provider), workspaceID); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("mysql: set selection: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("mysql: commit select-workspace tx: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (string, error) {
	var query string
	var args []any
	if workspaceID == "" {
		query = `SELECT credential_id FROM connections WHERE tenant_id=? AND user_id=? AND provider=? AND is_selected=true LIMIT 1`
		args = []any{tenantID, userID, string(provider)}
	} else {
		query = `SELECT credential_id FROM connections WHERE tenant_id=? AND user_id=? AND provider=? AND external_workspace_id=? LIMIT 1`
		args = []any{tenantID, userID, string(provider), workspaceID}
	}
	var credID string
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&credID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", usecase.ErrConnectionNotFound
		}
		return "", fmt.Errorf("mysql: get credential id: %w", err)
	}
	return credID, nil
}
