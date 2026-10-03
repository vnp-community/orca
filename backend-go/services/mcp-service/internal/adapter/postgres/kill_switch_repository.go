package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

const killColumns = `id::text, tenant_id::text, scope, target_id, active, reason, set_by::text, set_at`

func scanKill(row pgx.Row) (domain.KillSwitchEntry, error) {
	var e domain.KillSwitchEntry
	err := row.Scan(&e.ID, &e.TenantID, &e.Scope, &e.TargetID, &e.Active, &e.Reason, &e.SetBy, &e.SetAt)
	return e, err
}

func (r *Repository) UpsertKillSwitch(ctx context.Context, e domain.KillSwitchEntry, events []domain.OutboxRecord) (domain.KillSwitchEntry, error) {
	var out domain.KillSwitchEntry
	err := r.withTenantTx(ctx, e.TenantID, func(tx pgx.Tx) error {
		got, err := scanKill(tx.QueryRow(ctx, `
			INSERT INTO mcp.kill_switches (id, tenant_id, scope, target_id, active, reason, set_by, set_at, cleanup_pending)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $5 OR $3 = 'tenant')
			ON CONFLICT (tenant_id, scope, target_id) DO UPDATE SET active = EXCLUDED.active, reason = EXCLUDED.reason,
				set_by = EXCLUDED.set_by, set_at = EXCLUDED.set_at, cleanup_pending = EXCLUDED.active OR EXCLUDED.scope = 'tenant'
			RETURNING `+killColumns, e.ID, e.TenantID, e.Scope, e.TargetID, e.Active, e.Reason, e.SetBy, e.SetAt))
		if err != nil {
			return fmt.Errorf("postgres: upsert kill switch: %w", err)
		}
		if e.Scope == domain.KillScopeTenant {
			// Mirror into tenant_settings so GetServerInfo (which reads those
			// columns) reports the same state.
			at := any(nil)
			if e.Active {
				at = e.SetAt
			}
			if _, err := tx.Exec(ctx, `UPDATE mcp.tenant_settings SET kill_switch_active = $2, kill_switch_reason = $3, kill_switch_at = $4 WHERE tenant_id = $1`,
				e.TenantID, e.Active, e.Reason, at); err != nil {
				return fmt.Errorf("postgres: mirror kill switch: %w", err)
			}
		}
		if err := bumpEpochTx(ctx, tx, e.TenantID); err != nil {
			return err
		}
		out = got
		return insertEventsTx(ctx, tx, e.TenantID, events)
	})
	return out, err
}

func (r *Repository) ListKillSwitches(ctx context.Context, tenantID string, activeOnly bool) ([]domain.KillSwitchEntry, error) {
	var out []domain.KillSwitchEntry
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+killColumns+` FROM mcp.kill_switches WHERE tenant_id = $1 AND (NOT $2::bool OR active) ORDER BY set_at DESC, id`, tenantID, activeOnly)
		if err != nil {
			return fmt.Errorf("postgres: list kill switches: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanKill(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) PendingKillCleanups(ctx context.Context, limit int) ([]domain.KillSwitchEntry, error) {
	var out []domain.KillSwitchEntry
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+killColumns+` FROM mcp.kill_switches WHERE cleanup_pending ORDER BY set_at LIMIT $1`, limit)
		if err != nil {
			return fmt.Errorf("postgres: pending kill cleanups: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanKill(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) ClearKillCleanup(ctx context.Context, tenantID, id string) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE mcp.kill_switches SET cleanup_pending = false WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		return err
	})
}

func (r *Repository) GrantIDsInScope(ctx context.Context, tenantID, scope, target string) ([]string, error) {
	var out []string
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var q string
		args := []any{tenantID}
		switch scope {
		case domain.KillScopeTenant:
			q = `SELECT id::text FROM mcp.grants WHERE tenant_id = $1 AND status = 'active'`
		case domain.KillScopeClient:
			q = `SELECT id::text FROM mcp.grants WHERE tenant_id = $1 AND status = 'active' AND client_id = $2`
			args = append(args, target)
		case domain.KillScopeGrant:
			if _, err := uuid.Parse(target); err != nil {
				return nil
			}
			q = `SELECT id::text FROM mcp.grants WHERE tenant_id = $1 AND status = 'active' AND id = $2::uuid`
			args = append(args, target)
		default:
			return nil
		}
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("postgres: grants in scope: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) GrantExists(ctx context.Context, tenantID, grantID string) (bool, error) {
	if _, err := uuid.Parse(grantID); err != nil {
		return false, nil
	}
	var ok bool
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mcp.grants WHERE id = $1::uuid AND tenant_id = $2)`, grantID, tenantID).Scan(&ok)
	})
	return ok, err
}

var _ usecase.KillSwitchRepository = (*Repository)(nil)

// CountActiveKillSwitches counts active switches across tenants by scope for
// the orca_mcp_killswitch_active gauge (relay opt-in, read-only, no ids).
func (r *Repository) CountActiveKillSwitches(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT scope, count(*) FROM mcp.kill_switches WHERE active GROUP BY scope`)
		if err != nil {
			return fmt.Errorf("postgres: count active kill switches: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var scope string
			var n int64
			if err := rows.Scan(&scope, &n); err != nil {
				return err
			}
			out[scope] = n
		}
		return rows.Err()
	})
	return out, err
}
