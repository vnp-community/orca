package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

var _ usecase.ConnectionRepository = (*Repository)(nil)

func (r *Repository) Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO issuetracking.connections
			(tenant_id, user_id, provider, external_workspace_id, workspace_name, workspace_url,
			 viewer_id, viewer_display_name, viewer_email, credential_id, is_selected, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			NOT EXISTS (SELECT 1 FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3),
			now())
		ON CONFLICT (tenant_id, user_id, provider, external_workspace_id)
		DO UPDATE SET workspace_name = EXCLUDED.workspace_name, workspace_url = EXCLUDED.workspace_url,
			viewer_id = EXCLUDED.viewer_id, viewer_display_name = EXCLUDED.viewer_display_name,
			viewer_email = EXCLUDED.viewer_email, credential_id = EXCLUDED.credential_id, updated_at = now()
	`, tenantID, userID, string(provider), workspace.ID, workspace.Name, workspace.URL,
		viewer.ID, viewer.DisplayName, viewer.Email, credentialID)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: upsert connection: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error {
	if workspaceID == "" {
		_, err := r.pool.Exec(ctx, `DELETE FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3`, tenantID, userID, string(provider))
		if err != nil {
			return fmt.Errorf("postgres: delete all connections: %w", err)
		}
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4`,
		tenantID, userID, string(provider), workspaceID)
	if err != nil {
		return fmt.Errorf("postgres: delete connection: %w", err)
	}
	return nil
}

func (r *Repository) GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT external_workspace_id, workspace_name, workspace_url,
		       viewer_id, viewer_display_name, viewer_email, is_selected
		FROM issuetracking.connections
		WHERE tenant_id=$1 AND user_id=$2 AND provider=$3
		ORDER BY created_at
	`, tenantID, userID, string(provider))
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: query connection status: %w", err)
	}
	defer rows.Close()

	var status domain.ConnectionStatus
	for rows.Next() {
		var ws domain.Workspace
		var viewer domain.Viewer
		var selected bool
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.URL, &viewer.ID, &viewer.DisplayName, &viewer.Email, &selected); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("postgres: scan connection row: %w", err)
		}
		status.Workspaces = append(status.Workspaces, ws)
		if selected {
			status.SelectedWorkspaceID = ws.ID
			status.ActiveWorkspaceID = ws.ID
			status.Viewer = viewer
		}
	}
	if err := rows.Err(); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: iterate connection rows: %w", err)
	}
	status.Connected = len(status.Workspaces) > 0
	return status, nil
}

func (r *Repository) SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: begin select-workspace tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE issuetracking.connections SET is_selected=false WHERE tenant_id=$1 AND user_id=$2 AND provider=$3`,
		tenantID, userID, string(provider)); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: clear selection: %w", err)
	}
	if workspaceID != "" && workspaceID != "all" {
		if _, err := tx.Exec(ctx, `UPDATE issuetracking.connections SET is_selected=true WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4`,
			tenantID, userID, string(provider), workspaceID); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("postgres: set selection: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: commit select-workspace tx: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (string, error) {
	var query string
	var args []any
	if workspaceID == "" {
		query = `SELECT credential_id FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND is_selected=true LIMIT 1`
		args = []any{tenantID, userID, string(provider)}
	} else {
		query = `SELECT credential_id FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4 LIMIT 1`
		args = []any{tenantID, userID, string(provider), workspaceID}
	}
	var credID string
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&credID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", usecase.ErrConnectionNotFound
		}
		return "", fmt.Errorf("postgres: get credential id: %w", err)
	}
	return credID, nil
}
