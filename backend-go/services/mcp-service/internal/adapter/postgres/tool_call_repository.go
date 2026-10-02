package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Tool-call journal, sliding-window limiter and taint (BE-MCP-SOL-013 E/G).

const toolCallColumns = `id::text, tenant_id::text, user_id::text, client_id, client_name, COALESCE(mcp_session_id, ''),
	COALESCE(root_session_id, ''), tool_name, channel, risk, risk_class, params_hash, args_summary, decision, reason_code,
	COALESCE(approval_id::text, ''), COALESCE(approver::text, ''), state, COALESCE(result, ''), read_untrusted,
	COALESCE(duration_ms, 0), started_at, finished_at`

func scanToolCall(row pgx.Row) (domain.ToolCall, error) {
	var c domain.ToolCall
	err := row.Scan(&c.ID, &c.TenantID, &c.UserID, &c.ClientID, &c.ClientName, &c.SessionID, &c.RootSessionID, &c.ToolName, &c.Channel,
		&c.Risk, &c.RiskClass, &c.ParamsHash, &c.ArgsSummary, &c.Decision, &c.ReasonCode, &c.ApprovalID, &c.ApprovedBy, &c.State,
		&c.Result, &c.ReadUntrusted, &c.DurationMs, &c.StartedAt, &c.FinishedAt)
	return c, err
}

func insertCallTx(ctx context.Context, tx pgx.Tx, c domain.ToolCall) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO mcp.tool_calls (id, tenant_id, user_id, client_id, client_name, mcp_session_id, root_session_id, tool_name, channel, risk,
			risk_class, params_hash, args_summary, decision, reason_code, approval_id, approver, state, result, read_untrusted, duration_ms,
			started_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9, $10, $11, $12, $13, $14, $15, NULLIF($16, '')::uuid,
			NULLIF($17, '')::uuid, $18, NULLIF($19, ''), $20, $21, $22, $23)`,
		c.ID, c.TenantID, c.UserID, c.ClientID, c.ClientName, c.SessionID, c.RootSessionID, c.ToolName, c.Channel, c.Risk,
		c.RiskClass, c.ParamsHash, c.ArgsSummary, c.Decision, c.ReasonCode, c.ApprovalID, c.ApprovedBy, c.State, c.Result,
		c.ReadUntrusted, c.DurationMs, c.StartedAt, c.FinishedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert tool call: %w", err)
	}
	return nil
}

// insertFinalCallTx writes an already-final row plus its single audit event.
func insertFinalCallTx(ctx context.Context, tx pgx.Tx, c domain.ToolCall, now time.Time) error {
	c.State = domain.CallStateDone
	if c.FinishedAt == nil {
		c.FinishedAt = &now
	}
	if err := insertCallTx(ctx, tx, c); err != nil {
		return err
	}
	return emitCallAuditTx(ctx, tx, c, now)
}

func emitCallAuditTx(ctx context.Context, tx pgx.Tx, c domain.ToolCall, now time.Time) error {
	c.TraceID = traceIDFromContext(ctx)
	ev, err := domain.NewToolCallAuditEvent(uuid.NewString(), c, now, c.SuppressedCount)
	if err != nil {
		return fmt.Errorf("postgres: build audit event: %w", err)
	}
	return insertOutboxTx(ctx, tx, c.TenantID, ev)
}

func (r *Repository) RecordFinalCall(ctx context.Context, c domain.ToolCall, now time.Time) error {
	return r.withTenantTx(ctx, c.TenantID, func(tx pgx.Tx) error {
		return insertFinalCallTx(ctx, tx, c, now)
	})
}

func (r *Repository) AdmitToolCall(ctx context.Context, req usecase.AdmitRequest) (usecase.AdmitResult, error) {
	c := req.Call
	var res usecase.AdmitResult
	err := r.withTenantTx(ctx, c.TenantID, func(tx pgx.Tx) error {
		// One key per (tenant,user,client) serializes admission, so the
		// window counts are exact across replicas.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('rl:'||$1||':'||$2||':'||$3, 0))`,
			c.TenantID, c.UserID, c.ClientID); err != nil {
			return fmt.Errorf("postgres: limiter lock: %w", err)
		}
		var approvalID string
		if req.ConsumeApprovalHash != "" {
			err := tx.QueryRow(ctx, `
				SELECT id::text FROM mcp.approvals
				WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3 AND params_hash = $4
				  AND status = 'approved' AND consumed_at IS NULL AND expires_at > $5
				FOR UPDATE`, c.TenantID, c.UserID, c.ClientID, req.ConsumeApprovalHash, req.Now).Scan(&approvalID)
			if errors.Is(err, pgx.ErrNoRows) {
				res.ApprovalMissing = true
				return nil
			}
			if err != nil {
				return fmt.Errorf("postgres: find approved approval: %w", err)
			}
		}
		if reason, err := checkLimits(ctx, tx, c, req.Limits, req.Now); err != nil {
			return err
		} else if reason != "" {
			res.DenyReason = reason
			return nil
		}
		if approvalID != "" {
			tag, err := tx.Exec(ctx, `UPDATE mcp.approvals SET consumed_at = $2, consumed_call_id = $3
				WHERE id = $1 AND status = 'approved' AND consumed_at IS NULL`, approvalID, req.Now, c.ID)
			if err != nil {
				return fmt.Errorf("postgres: consume approval: %w", err)
			}
			if tag.RowsAffected() != 1 { // unreachable under FOR UPDATE; kept as a single-use backstop
				res.ApprovalMissing = true
				return nil
			}
			c.ApprovalID, c.ApprovedBy = approvalID, c.UserID
		}
		c.StartedAt, c.State = req.Now, domain.CallStateStarted
		if err := insertCallTx(ctx, tx, c); err != nil {
			return err
		}
		res.Admitted, res.ApprovalID, res.ApproverID = true, approvalID, c.ApprovedBy
		return nil
	})
	return res, err
}

func checkLimits(ctx context.Context, tx pgx.Tx, c domain.ToolCall, l usecase.RateLimits, now time.Time) (string, error) {
	var total, inClass, daily, loop1m, loop5m int
	err := tx.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE client_id = $3 AND started_at > $5::timestamptz - interval '60 seconds'),
		  count(*) FILTER (WHERE client_id = $3 AND risk_class = $4 AND started_at > $5::timestamptz - interval '60 seconds'),
		  count(*),
		  count(*) FILTER (WHERE client_id = $3 AND params_hash = $6 AND started_at > $5::timestamptz - interval '60 seconds'),
		  count(*) FILTER (WHERE client_id = $3 AND params_hash = $6 AND started_at > $5::timestamptz - interval '5 minutes')
		FROM mcp.tool_calls
		WHERE tenant_id = $1 AND user_id = $2 AND decision IN ('allow', 'approved') AND started_at > $5::timestamptz - interval '24 hours'`,
		c.TenantID, c.UserID, c.ClientID, c.RiskClass, now, c.ParamsHash).Scan(&total, &inClass, &daily, &loop1m, &loop5m)
	if err != nil {
		return "", fmt.Errorf("postgres: count tool calls: %w", err)
	}
	switch {
	case l.LoopBlock > 0 && loop5m >= l.LoopBlock:
		return domain.ReasonLoopBlocked, nil
	case l.LoopSlowDown > 0 && loop1m >= l.LoopSlowDown:
		return domain.ReasonLoopSlowDown, nil
	case l.TotalPerMinute > 0 && total >= l.TotalPerMinute:
		return domain.ReasonRateLimited, nil
	case l.PerMinute[c.RiskClass] > 0 && inClass >= l.PerMinute[c.RiskClass]:
		return domain.ReasonRateLimited, nil
	case l.DailyPerUser > 0 && daily >= l.DailyPerUser:
		return domain.ReasonRateLimited, nil
	}
	return "", nil
}

func (r *Repository) FinalizeToolCall(ctx context.Context, tenantID, userID, callID, result, reason string, durationMs int64, now time.Time, taintTTL time.Duration) (domain.ToolCall, error) {
	var out domain.ToolCall
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		c, err := scanToolCall(tx.QueryRow(ctx, `
			UPDATE mcp.tool_calls SET state = 'done', result = $4, duration_ms = $5, finished_at = $6,
				reason_code = CASE WHEN $7 <> '' THEN $7 ELSE reason_code END
			WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND state = 'started'
			RETURNING `+toolCallColumns, callID, tenantID, userID, result, durationMs, now, reason))
		if errors.Is(err, pgx.ErrNoRows) {
			// Completing twice is harmless; an unknown/foreign id is not found.
			cur, serr := scanToolCall(tx.QueryRow(ctx, `SELECT `+toolCallColumns+` FROM mcp.tool_calls WHERE id = $1 AND tenant_id = $2 AND user_id = $3`,
				callID, tenantID, userID))
			if errors.Is(serr, pgx.ErrNoRows) {
				return domain.ErrNotFound()
			}
			out = cur
			return serr
		}
		if err != nil {
			return fmt.Errorf("postgres: finalize tool call: %w", err)
		}
		if c.ReadUntrusted && result == domain.ResultOK {
			if _, err := tx.Exec(ctx, `
				INSERT INTO mcp.taint (tenant_id, user_id, client_id, tainted_until) VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, user_id, client_id) DO UPDATE SET tainted_until = EXCLUDED.tainted_until`,
				tenantID, userID, c.ClientID, now.Add(taintTTL)); err != nil {
				return fmt.Errorf("postgres: upsert taint: %w", err)
			}
		}
		out = c
		return emitCallAuditTx(ctx, tx, c, now)
	})
	return out, err
}

func (r *Repository) IsTainted(ctx context.Context, tenantID, userID, clientID string, now time.Time) (bool, error) {
	var tainted bool
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mcp.taint WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3 AND tainted_until > $4)`,
			tenantID, userID, clientID, now).Scan(&tainted)
	})
	return tainted, err
}

func (r *Repository) ListStaleCalls(ctx context.Context, startedBefore time.Time, limit int) ([]usecase.Ref, error) {
	var out []usecase.Ref
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT tenant_id::text, id::text FROM mcp.tool_calls WHERE state = 'started' AND started_at < $1 ORDER BY started_at LIMIT $2`, startedBefore, limit)
		if err != nil {
			return fmt.Errorf("postgres: list stale calls: %w", err)
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

func interruptTx(ctx context.Context, tx pgx.Tx, c domain.ToolCall, reason string, now time.Time) error {
	c.State, c.Result, c.ReasonCode, c.FinishedAt = domain.CallStateDone, domain.ResultError, reason, &now
	return emitCallAuditTx(ctx, tx, c, now)
}

func (r *Repository) InterruptCall(ctx context.Context, tenantID, callID, reason string, now time.Time) (bool, error) {
	done := false
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		c, err := scanToolCall(tx.QueryRow(ctx, `
			UPDATE mcp.tool_calls SET state = 'done', result = 'error', reason_code = $3, finished_at = $4
			WHERE id = $1 AND tenant_id = $2 AND state = 'started' RETURNING `+toolCallColumns, callID, tenantID, reason, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: interrupt call: %w", err)
		}
		done = true
		return interruptTx(ctx, tx, c, reason, now)
	})
	return done, err
}

func (r *Repository) InterruptCallsInScope(ctx context.Context, tenantID, scope, target, reason string, now time.Time) ([]string, error) {
	var ids []string
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		pred, extra, ok := scopeFilter(scope, target, 4, true)
		if !ok {
			return nil
		}
		args := append([]any{tenantID, reason, now}, extra...)
		rows, err := tx.Query(ctx, `
			UPDATE mcp.tool_calls SET state = 'done', result = 'error', reason_code = $2, finished_at = $3
			WHERE tenant_id = $1 AND state = 'started' AND `+pred+` RETURNING `+toolCallColumns, args...)
		if err != nil {
			return fmt.Errorf("postgres: interrupt calls in scope: %w", err)
		}
		var calls []domain.ToolCall
		for rows.Next() {
			c, err := scanToolCall(rows)
			if err != nil {
				rows.Close()
				return err
			}
			calls = append(calls, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, c := range calls {
			if err := interruptTx(ctx, tx, c, reason, now); err != nil {
				return err
			}
			ids = append(ids, c.ID)
		}
		return nil
	})
	return ids, err
}

func (r *Repository) PurgeFinishedCalls(ctx context.Context, before time.Time, limit int) (int, error) {
	n := 0
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM mcp.tool_calls WHERE id IN (
			SELECT id FROM mcp.tool_calls WHERE state = 'done' AND started_at < $1 LIMIT $2)`, before, limit)
		if err != nil {
			return fmt.Errorf("postgres: purge tool calls: %w", err)
		}
		n = int(tag.RowsAffected())
		return nil
	})
	return n, err
}

var _ usecase.ToolCallRepository = (*Repository)(nil)
