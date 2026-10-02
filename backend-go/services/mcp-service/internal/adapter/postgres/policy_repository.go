package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Tool policies and tenant settings (BE-MCP-SOL-012). Every mutation bumps
// policy_epoch and enqueues its events in the same transaction.

const policyColumns = `id::text, version, COALESCE(match_tool, ''), COALESCE(match_namespace, ''), COALESCE(match_risk, ''),
	COALESCE(match_client_id, ''), COALESCE(match_roles, '{}'), decision, COALESCE(note, ''), created_by::text, updated_by::text, updated_at`

func scanPolicy(row pgx.Row) (domain.ToolPolicy, error) {
	var p domain.ToolPolicy
	err := row.Scan(&p.ID, &p.Version, &p.Match.Tool, &p.Match.Namespace, &p.Match.Risk, &p.Match.ClientID, &p.Match.Roles,
		&p.Decision, &p.Note, &p.CreatedBy, &p.UpdatedBy, &p.UpdatedAt)
	return p, err
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func rolesArg(r []string) any {
	if len(r) == 0 {
		return nil
	}
	return r
}

func insertEventsTx(ctx context.Context, tx pgx.Tx, tenantID string, events []domain.OutboxRecord) error {
	for _, ev := range events {
		if err := insertOutboxTx(ctx, tx, tenantID, ev); err != nil {
			return err
		}
	}
	return nil
}

func bumpEpochTx(ctx context.Context, tx pgx.Tx, tenantID string) error {
	// UPDATE only: never create a settings row here, that would bypass the
	// explicit MCP_TENANT_DEFAULT_ENABLED insert (D6).
	if _, err := tx.Exec(ctx, `UPDATE mcp.tenant_settings SET policy_epoch = policy_epoch + 1 WHERE tenant_id = $1`, tenantID); err != nil {
		return fmt.Errorf("postgres: bump policy epoch: %w", err)
	}
	return nil
}

func insertRevisionTx(ctx context.Context, tx pgx.Tx, tenantID string, p domain.ToolPolicy, op, actor string) error {
	snap, err := json.Marshal(map[string]any{"id": p.ID, "match": p.Match.AsMap(), "decision": p.Decision, "note": p.Note})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO mcp.tool_policy_revisions (policy_id, tenant_id, version, op, snapshot, changed_by)
		VALUES ($1, $2, $3, $4, $5, $6)`, p.ID, tenantID, p.Version, op, snap, actor)
	if err != nil {
		return fmt.Errorf("postgres: insert policy revision: %w", err)
	}
	return nil
}

const settingsWithEpochColumns = tenantSettingsColumns + `, policy_epoch`

func scanSettingsWithEpoch(row pgx.Row) (domain.TenantSettings, int64, error) {
	var s domain.TenantSettings
	var epoch int64
	err := row.Scan(&s.TenantID, &s.Enabled, &s.DCREnabled, &s.MaxTokenDays, &s.ApprovalTTLSeconds,
		&s.KillSwitch.Active, &s.KillSwitch.Reason, &s.KillSwitch.At, &s.UpdatedBy, &s.UpdatedAt, &epoch)
	return s, epoch, err
}

func (r *Repository) LoadPolicySnapshot(ctx context.Context, d domain.TenantSettings) (usecase.PolicySnapshot, error) {
	var out usecase.PolicySnapshot
	err := r.withTenantTx(ctx, d.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.tenant_settings (tenant_id, enabled, dcr_enabled, max_token_days, approval_ttl_seconds)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (tenant_id) DO NOTHING`,
			d.TenantID, d.Enabled, d.DCREnabled, d.MaxTokenDays, d.ApprovalTTLSeconds); err != nil {
			return fmt.Errorf("postgres: insert default tenant settings: %w", err)
		}
		s, epoch, err := scanSettingsWithEpoch(tx.QueryRow(ctx,
			`SELECT `+settingsWithEpochColumns+` FROM mcp.tenant_settings WHERE tenant_id = $1`, d.TenantID))
		if err != nil {
			return fmt.Errorf("postgres: select tenant settings: %w", err)
		}
		pols, err := listPoliciesTx(ctx, tx, d.TenantID)
		if err != nil {
			return err
		}
		out = usecase.PolicySnapshot{Settings: s, Policies: pols, Epoch: epoch}
		return nil
	})
	return out, err
}

func listPoliciesTx(ctx context.Context, tx pgx.Tx, tenantID string) ([]domain.ToolPolicy, error) {
	rows, err := tx.Query(ctx, `SELECT `+policyColumns+` FROM mcp.tool_policies WHERE tenant_id = $1 ORDER BY created_at, id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list tool policies: %w", err)
	}
	defer rows.Close()
	out := []domain.ToolPolicy{}
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan tool policy: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) ListToolPolicies(ctx context.Context, tenantID string) ([]domain.ToolPolicy, error) {
	var out []domain.ToolPolicy
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) (err error) {
		out, err = listPoliciesTx(ctx, tx, tenantID)
		return err
	})
	return out, err
}

func (r *Repository) CreateToolPolicy(ctx context.Context, tenantID string, p domain.ToolPolicy, events []domain.OutboxRecord) (domain.ToolPolicy, error) {
	var out domain.ToolPolicy
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		got, err := scanPolicy(tx.QueryRow(ctx, `
			INSERT INTO mcp.tool_policies (id, tenant_id, version, match_tool, match_namespace, match_risk, match_client_id, match_roles,
				decision, note, created_by, updated_by, created_at, updated_at)
			VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11, $11)
			RETURNING `+policyColumns,
			p.ID, tenantID, nilIfEmpty(p.Match.Tool), nilIfEmpty(p.Match.Namespace), nilIfEmpty(p.Match.Risk), nilIfEmpty(p.Match.ClientID),
			rolesArg(p.Match.Roles), p.Decision, nilIfEmpty(p.Note), p.UpdatedBy, p.UpdatedAt))
		if err != nil {
			return fmt.Errorf("postgres: insert tool policy: %w", err)
		}
		if err := insertRevisionTx(ctx, tx, tenantID, got, "create", p.UpdatedBy); err != nil {
			return err
		}
		if err := bumpEpochTx(ctx, tx, tenantID); err != nil {
			return err
		}
		out = got
		return insertEventsTx(ctx, tx, tenantID, events)
	})
	return out, err
}

func (r *Repository) UpdateToolPolicy(ctx context.Context, tenantID string, p domain.ToolPolicy, events []domain.OutboxRecord) (domain.ToolPolicy, error) {
	var out domain.ToolPolicy
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		got, err := scanPolicy(tx.QueryRow(ctx, `
			UPDATE mcp.tool_policies SET version = version + 1, match_tool = $4, match_namespace = $5, match_risk = $6,
				match_client_id = $7, match_roles = $8, decision = $9, note = $10, updated_by = $11, updated_at = $12
			WHERE id = $1 AND tenant_id = $2 AND version = $3
			RETURNING `+policyColumns,
			p.ID, tenantID, p.Version, nilIfEmpty(p.Match.Tool), nilIfEmpty(p.Match.Namespace), nilIfEmpty(p.Match.Risk),
			nilIfEmpty(p.Match.ClientID), rolesArg(p.Match.Roles), p.Decision, nilIfEmpty(p.Note), p.UpdatedBy, p.UpdatedAt))
		if errors.Is(err, pgx.ErrNoRows) {
			var current int
			qerr := tx.QueryRow(ctx, `SELECT version FROM mcp.tool_policies WHERE id = $1 AND tenant_id = $2`, p.ID, tenantID).Scan(&current)
			if errors.Is(qerr, pgx.ErrNoRows) {
				return domain.ErrNotFound()
			}
			if qerr != nil {
				return fmt.Errorf("postgres: read policy version: %w", qerr)
			}
			return domain.ErrPolicyVersionConflict(current)
		}
		if err != nil {
			return fmt.Errorf("postgres: update tool policy: %w", err)
		}
		if err := insertRevisionTx(ctx, tx, tenantID, got, "update", p.UpdatedBy); err != nil {
			return err
		}
		if err := bumpEpochTx(ctx, tx, tenantID); err != nil {
			return err
		}
		out = got
		return insertEventsTx(ctx, tx, tenantID, events)
	})
	return out, err
}

func (r *Repository) DeleteToolPolicy(ctx context.Context, tenantID, id, actor string, events []domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		cur, err := scanPolicy(tx.QueryRow(ctx, `SELECT `+policyColumns+` FROM mcp.tool_policies WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound()
		}
		if err != nil {
			return fmt.Errorf("postgres: load policy for delete: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM mcp.tool_policies WHERE id = $1 AND tenant_id = $2`, id, tenantID); err != nil {
			return fmt.Errorf("postgres: delete tool policy: %w", err)
		}
		if err := insertRevisionTx(ctx, tx, tenantID, cur, "delete", actor); err != nil {
			return err
		}
		if err := bumpEpochTx(ctx, tx, tenantID); err != nil {
			return err
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}

func (r *Repository) PatchTenantSettings(ctx context.Context, d domain.TenantSettings, patch usecase.SettingsPatch, actor string, events []domain.OutboxRecord) (domain.TenantSettings, error) {
	var out domain.TenantSettings
	err := r.withTenantTx(ctx, d.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.tenant_settings (tenant_id, enabled, dcr_enabled, max_token_days, approval_ttl_seconds)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (tenant_id) DO NOTHING`,
			d.TenantID, d.Enabled, d.DCREnabled, d.MaxTokenDays, d.ApprovalTTLSeconds); err != nil {
			return fmt.Errorf("postgres: insert default tenant settings: %w", err)
		}
		s, _, err := scanSettingsWithEpoch(tx.QueryRow(ctx, `
			UPDATE mcp.tenant_settings SET enabled = COALESCE($2, enabled), dcr_enabled = COALESCE($3, dcr_enabled),
				max_token_days = COALESCE($4, max_token_days), approval_ttl_seconds = COALESCE($5, approval_ttl_seconds),
				updated_by = NULLIF($6, '')::uuid, updated_at = now(), policy_epoch = policy_epoch + 1
			WHERE tenant_id = $1
			RETURNING `+settingsWithEpochColumns,
			d.TenantID, patch.Enabled, patch.DCREnabled, patch.MaxTokenDays, patch.ApprovalTTLSeconds, actor))
		if err != nil {
			return fmt.Errorf("postgres: patch tenant settings: %w", err)
		}
		out = s
		return insertEventsTx(ctx, tx, d.TenantID, events)
	})
	return out, err
}

var _ usecase.PolicyRepository = (*Repository)(nil)
