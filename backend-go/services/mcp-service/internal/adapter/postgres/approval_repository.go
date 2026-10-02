package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Approvals (BE-MCP-SOL-013 B). Single-use is enforced in SQL: consumption
// and decisions are conditional UPDATEs, never read-then-write.

const approvalColumns = `id::text, tenant_id::text, user_id::text, client_id, client_name, COALESCE(mcp_session_id, ''),
	COALESCE(call_id::text, ''), tool_name, tool_title, channel, risk, params_hash, args_preview, args_redacted, reasons,
	status, created_at, expires_at, decided_at, COALESCE(decided_via, ''), COALESCE(decision_note, ''), consumed_at`

func scanApproval(row pgx.Row) (domain.Approval, error) {
	var a domain.Approval
	err := row.Scan(&a.ID, &a.TenantID, &a.UserID, &a.ClientID, &a.ClientName, &a.SessionID, &a.CallID, &a.ToolName, &a.ToolTitle,
		&a.Channel, &a.Risk, &a.ParamsHash, &a.ArgsPreview, &a.ArgsRedacted, &a.Reasons, &a.Status, &a.CreatedAt, &a.ExpiresAt,
		&a.DecidedAt, &a.DecidedVia, &a.DecisionNote, &a.ConsumedAt)
	return a, err
}

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func resolvedEvent(a domain.Approval, now time.Time) (domain.OutboxRecord, error) {
	return domain.NewApprovalResolvedEvent(uuid.NewString(), a, now)
}

// finalCallFromApproval builds the journal row for an approval that ended
// without executing (denied / expired), so the audit trail shows the outcome.
func finalCallFromApproval(a domain.Approval, decision, approver string, now time.Time) domain.ToolCall {
	fin := now
	return domain.ToolCall{
		ID: uuid.NewString(), TenantID: a.TenantID, UserID: a.UserID, ClientID: a.ClientID, ClientName: a.ClientName,
		SessionID: a.SessionID, ToolName: a.ToolName, Channel: a.Channel, Risk: a.Risk, RiskClass: domain.RiskClass(a.Risk),
		ParamsHash: a.ParamsHash, ArgsSummary: domain.SummarizeArgs(a.ArgsPreview, domain.SecretRedactor{}), Decision: decision,
		ApprovalID: a.ID, ApprovedBy: approver, State: domain.CallStateDone, StartedAt: now, FinishedAt: &fin,
		ReasonCode: map[string]string{domain.CallExpired: domain.ReasonApprovalExpired}[decision],
	}
}

func (r *Repository) FindOrCreatePendingApproval(ctx context.Context, a domain.Approval, limits usecase.ApprovalLimits, now time.Time) (domain.Approval, bool, error) {
	var out domain.Approval
	created := false
	err := r.withTenantTx(ctx, a.TenantID, func(tx pgx.Tx) error {
		// Serialize creators of the same (user, client) so the flood caps are exact.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('ap:'||$1||':'||$2||':'||$3, 0))`,
			a.TenantID, a.UserID, a.ClientID); err != nil {
			return fmt.Errorf("postgres: approval lock: %w", err)
		}
		existing, err := scanApproval(tx.QueryRow(ctx, `
			SELECT `+approvalColumns+` FROM mcp.approvals
			WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3 AND params_hash = $4
			  AND status IN ('pending', 'approved') AND consumed_at IS NULL FOR UPDATE`,
			a.TenantID, a.UserID, a.ClientID, a.ParamsHash))
		switch {
		case err == nil && existing.ExpiresAt.After(now):
			out = existing
			return nil
		case err == nil:
			if err := expireTx(ctx, tx, existing, now); err != nil {
				return err
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("postgres: find open approval: %w", err)
		}
		var pending, recent int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE status = 'pending' AND expires_at > $4::timestamptz),
			       count(*) FILTER (WHERE created_at > $4::timestamptz - interval '1 hour')
			FROM mcp.approvals WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3`,
			a.TenantID, a.UserID, a.ClientID, now).Scan(&pending, &recent); err != nil {
			return fmt.Errorf("postgres: count approvals: %w", err)
		}
		if (limits.MaxPendingPerClient > 0 && pending >= limits.MaxPendingPerClient) ||
			(limits.MaxCreatedPerHour > 0 && recent >= limits.MaxCreatedPerHour) {
			return usecase.ErrApprovalFlood
		}
		got, err := scanApproval(tx.QueryRow(ctx, `
			INSERT INTO mcp.approvals (id, tenant_id, user_id, client_id, client_name, mcp_session_id, tool_name, tool_title, channel, risk,
				params_hash, args_preview, args_redacted, reasons, status, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13, $14, 'pending', $15, $16)
			RETURNING `+approvalColumns,
			a.ID, a.TenantID, a.UserID, a.ClientID, a.ClientName, a.SessionID, a.ToolName, a.ToolTitle, a.Channel, a.Risk,
			a.ParamsHash, a.ArgsPreview, a.ArgsRedacted, nonNilStrings(a.Reasons), a.CreatedAt, a.ExpiresAt))
		if err != nil {
			return fmt.Errorf("postgres: insert approval: %w", err)
		}
		ev, err := domain.NewApprovalRequestedEvent(uuid.NewString(), got, now)
		if err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, a.TenantID, ev); err != nil {
			return err
		}
		out, created = got, true
		return nil
	})
	if isUniqueViolation(err) { // lost a race despite the lock (lock is per client; index is global): reuse the winner
		return r.findOpenApproval(ctx, a)
	}
	return out, created, err
}

func (r *Repository) findOpenApproval(ctx context.Context, a domain.Approval) (domain.Approval, bool, error) {
	var out domain.Approval
	err := r.withTenantTx(ctx, a.TenantID, func(tx pgx.Tx) (err error) {
		out, err = scanApproval(tx.QueryRow(ctx, `
			SELECT `+approvalColumns+` FROM mcp.approvals
			WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3 AND params_hash = $4
			  AND status IN ('pending', 'approved') AND consumed_at IS NULL`, a.TenantID, a.UserID, a.ClientID, a.ParamsHash))
		return err
	})
	return out, false, err
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// expireTx moves one open approval to expired with its resolved event and, for
// never-decided approvals, a journal row (decision=expired).
func expireTx(ctx context.Context, tx pgx.Tx, a domain.Approval, now time.Time) error {
	tag, err := tx.Exec(ctx, `UPDATE mcp.approvals SET status = 'expired' WHERE id = $1 AND tenant_id = $2 AND status IN ('pending', 'approved') AND consumed_at IS NULL`,
		a.ID, a.TenantID)
	if err != nil {
		return fmt.Errorf("postgres: expire approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	a.Status = domain.ApprovalExpired
	ev, err := resolvedEvent(a, now)
	if err != nil {
		return err
	}
	if err := insertOutboxTx(ctx, tx, a.TenantID, ev); err != nil {
		return err
	}
	return insertFinalCallTx(ctx, tx, finalCallFromApproval(a, domain.CallExpired, "", now), now)
}

func (r *Repository) DecideApproval(ctx context.Context, in usecase.DecideApprovalRepoInput) (domain.Approval, error) {
	var out domain.Approval
	newStatus := domain.ApprovalDenied
	if in.Approve {
		newStatus = domain.ApprovalApproved
	}
	err := r.withTenantTx(ctx, in.TenantID, func(tx pgx.Tx) error {
		a, err := scanApproval(tx.QueryRow(ctx, `
			UPDATE mcp.approvals SET status = $5, decided_at = $6, decided_via = $7, decision_note = NULLIF($8, '')
			WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status = 'pending' AND expires_at > $6
			  AND ($5 = 'denied' OR params_hash = $4)
			RETURNING `+approvalColumns, in.ApprovalID, in.TenantID, in.UserID, in.ParamsHash, newStatus, in.Now, in.Via, in.Note))
		if errors.Is(err, pgx.ErrNoRows) {
			cur, serr := scanApproval(tx.QueryRow(ctx, `SELECT `+approvalColumns+` FROM mcp.approvals WHERE id = $1 AND tenant_id = $2 AND user_id = $3`,
				in.ApprovalID, in.TenantID, in.UserID))
			if errors.Is(serr, pgx.ErrNoRows) {
				return domain.DecideDiagnosis(domain.Approval{}, false, in.Now)
			}
			if serr != nil {
				return fmt.Errorf("postgres: diagnose approval: %w", serr)
			}
			return domain.DecideDiagnosis(cur, true, in.Now)
		}
		if err != nil {
			return fmt.Errorf("postgres: decide approval: %w", err)
		}
		ev, err := resolvedEvent(a, in.Now)
		if err != nil {
			return err
		}
		if err := insertOutboxTx(ctx, tx, in.TenantID, ev); err != nil {
			return err
		}
		if in.Approve {
			audit, err := domain.NewAdminAuditEvent(uuid.NewString(), uuid.NewString(), in.TenantID, in.UserID, domain.AuditActionApprovalDecide,
				"mcp_approval", a.ID, "allowed", in.Now, map[string]any{
					"approval_id": a.ID, "tool": a.ToolName, "risk": a.Risk, "decided_via": in.Via, "args_hash": a.ParamsHash, "client_name": a.ClientName,
				})
			if err != nil {
				return err
			}
			if err := insertOutboxTx(ctx, tx, in.TenantID, audit); err != nil {
				return err
			}
		} else if err := insertFinalCallTx(ctx, tx, finalCallFromApproval(a, domain.CallDenied, in.UserID, in.Now), in.Now); err != nil {
			return err
		}
		out = a
		return nil
	})
	return out, err
}

func (r *Repository) GetApproval(ctx context.Context, tenantID, id string) (domain.Approval, error) {
	var out domain.Approval
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		a, err := scanApproval(tx.QueryRow(ctx, `SELECT `+approvalColumns+` FROM mcp.approvals WHERE id = $1 AND tenant_id = $2`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound()
		}
		out = a
		return err
	})
	return out, err
}

func (r *Repository) ListApprovals(ctx context.Context, q usecase.ApprovalQuery) ([]domain.Approval, error) {
	var out []domain.Approval
	var cursorAt *time.Time
	var cursorID *string
	if !q.CursorAt.IsZero() {
		cursorAt, cursorID = &q.CursorAt, &q.CursorID
	}
	err := r.withTenantTx(ctx, q.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+approvalColumns+` FROM mcp.approvals
			WHERE tenant_id = $1 AND user_id = $2
			  AND (NOT $3::bool OR (status = 'pending' AND expires_at > $4::timestamptz))
			  AND ($5::timestamptz IS NULL OR (created_at, id) < ($5::timestamptz, $6::uuid))
			ORDER BY created_at DESC, id DESC LIMIT $7`,
			q.TenantID, q.UserID, q.PendingOnly, q.Now, cursorAt, cursorID, q.Limit)
		if err != nil {
			return fmt.Errorf("postgres: list approvals: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			a, err := scanApproval(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan approval: %w", err)
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) ListExpiredApprovalRefs(ctx context.Context, now time.Time, limit int) ([]usecase.Ref, error) {
	var out []usecase.Ref
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id::text, id::text FROM mcp.approvals WHERE status = 'pending' AND expires_at <= $1 ORDER BY expires_at LIMIT $2`, now, limit)
		if err != nil {
			return fmt.Errorf("postgres: list expired approvals: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var ref usecase.Ref
			if err := rows.Scan(&ref.TenantID, &ref.ID); err != nil {
				return err
			}
			out = append(out, ref)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) ExpireApproval(ctx context.Context, tenantID, id string, now time.Time) (bool, error) {
	expired := false
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		a, err := scanApproval(tx.QueryRow(ctx, `SELECT `+approvalColumns+` FROM mcp.approvals
			WHERE id = $1 AND tenant_id = $2 AND status = 'pending' AND expires_at <= $3 FOR UPDATE`, id, tenantID, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already handled by another replica or decided meanwhile
		}
		if err != nil {
			return fmt.Errorf("postgres: lock approval: %w", err)
		}
		expired = true
		return expireTx(ctx, tx, a, now)
	})
	return expired, err
}

// scopeFilter builds the SQL predicate for a kill-switch scope over a table
// that has client_id, user_id, mcp_session_id (and optionally root_session_id).
func scopeFilter(scope, target string, argPos int, withRoot bool) (string, []any, bool) {
	p := fmt.Sprintf("$%d", argPos)
	switch scope {
	case domain.KillScopeTenant:
		return "TRUE", nil, true
	case domain.KillScopeClient:
		return "client_id = " + p, []any{target}, true
	case domain.KillScopeSession:
		if withRoot {
			return "(mcp_session_id = " + p + " OR root_session_id = " + p + ")", []any{target}, true
		}
		return "mcp_session_id = " + p, []any{target}, true
	case domain.KillScopeGrant:
		if _, err := uuid.Parse(target); err != nil {
			return "", nil, false
		}
		return "(user_id, client_id) IN (SELECT user_id, client_id FROM mcp.grants WHERE id = " + p + "::uuid AND tenant_id = $1)", []any{target}, true
	}
	return "", nil, false
}

func (r *Repository) CancelApprovals(ctx context.Context, tenantID, scope, target string, now time.Time) (int, error) {
	n := 0
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		pred, extra, ok := scopeFilter(scope, target, 2, false)
		if !ok {
			return nil
		}
		args := append([]any{tenantID}, extra...)
		rows, err := tx.Query(ctx, `UPDATE mcp.approvals SET status = 'cancelled'
			WHERE tenant_id = $1 AND status IN ('pending', 'approved') AND consumed_at IS NULL AND `+pred+`
			RETURNING `+approvalColumns, args...)
		if err != nil {
			return fmt.Errorf("postgres: cancel approvals: %w", err)
		}
		var cancelled []domain.Approval
		for rows.Next() {
			a, err := scanApproval(rows)
			if err != nil {
				rows.Close()
				return err
			}
			cancelled = append(cancelled, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, a := range cancelled {
			ev, err := resolvedEvent(a, now)
			if err != nil {
				return err
			}
			if err := insertOutboxTx(ctx, tx, tenantID, ev); err != nil {
				return err
			}
		}
		n = len(cancelled)
		return nil
	})
	return n, err
}

var _ usecase.ApprovalRepository = (*Repository)(nil)
