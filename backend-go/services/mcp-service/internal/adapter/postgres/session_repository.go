package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const sessionColumns = `s.id::text, s.tenant_id::text, s.user_id::text, s.secret_hash, s.client_id, s.client_name, s.client_version,
	s.grant_id, s.token_id, s.protocol_version, s.capabilities::text, s.log_level, s.state, s.tool_calls,
	s.created_at, s.last_seen_at, COALESCE(s.close_reason, '')`

func scanSession(row pgx.Row, withStreams bool) (domain.Session, error) {
	var s domain.Session
	var caps string
	dst := []any{&s.ID, &s.TenantID, &s.UserID, &s.SecretHash, &s.ClientID, &s.ClientName, &s.ClientVersion,
		&s.GrantID, &s.TokenID, &s.ProtocolVersion, &caps, &s.LogLevel, &s.State, &s.ToolCalls,
		&s.CreatedAt, &s.LastSeenAt, &s.CloseReason}
	if withStreams {
		dst = append(dst, &s.ActiveStreams)
	}
	if err := row.Scan(dst...); err != nil {
		return domain.Session{}, err
	}
	s.CapabilitiesJSON = []byte(caps)
	return s, nil
}

func notFoundIfNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound()
	}
	return err
}

func (r *Repository) CreateSession(ctx context.Context, s domain.Session) (domain.Session, error) {
	var out domain.Session
	err := r.withTenantTx(ctx, s.TenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO mcp.sessions AS s (id, tenant_id, user_id, secret_hash, client_id, client_name, client_version, grant_id, token_id,
				protocol_version, capabilities, log_level, state, created_at, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $14)
			RETURNING `+sessionColumns, s.ID, s.TenantID, s.UserID, s.SecretHash, s.ClientID, s.ClientName, s.ClientVersion, s.GrantID, s.TokenID,
			s.ProtocolVersion, string(s.CapabilitiesJSON), s.LogLevel, s.State, s.CreatedAt)
		got, err := scanSession(row, false)
		if err != nil {
			return fmt.Errorf("postgres: create session: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *Repository) GetSessionBySecretHash(ctx context.Context, tenantID string, hash []byte) (domain.Session, error) {
	var out domain.Session
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		got, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionColumns+` FROM mcp.sessions s WHERE s.tenant_id = $1 AND s.secret_hash = $2`, tenantID, hash), false)
		if err != nil {
			return notFoundIfNoRows(fmt.Errorf("postgres: get session by secret: %w", err))
		}
		out = got
		return nil
	})
	return out, unwrapNotFound(err)
}

// unwrapNotFound turns a wrapped pgx.ErrNoRows into the domain not-found error.
func unwrapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound()
	}
	return err
}

func (r *Repository) GetSession(ctx context.Context, tenantID, id string) (domain.Session, error) {
	var out domain.Session
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		got, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionColumns+` FROM mcp.sessions s WHERE s.tenant_id = $1 AND s.id::text = $2`, tenantID, id), false)
		if err != nil {
			return fmt.Errorf("postgres: get session: %w", err)
		}
		out = got
		return nil
	})
	return out, unwrapNotFound(err)
}

func (r *Repository) TouchSession(ctx context.Context, tenantID, id string, ready bool, delta int64, logLevel string, now time.Time) (string, error) {
	var state string
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// A closed session is never revived: the CASE keeps state and the WHERE
		// lets the caller learn "closed" from a second query.
		err := tx.QueryRow(ctx, `
			UPDATE mcp.sessions SET last_seen_at = $3,
				tool_calls = tool_calls + $4,
				state = CASE WHEN $5 AND state = 'initializing' THEN 'ready' ELSE state END,
				log_level = CASE WHEN $6 <> '' THEN $6 ELSE log_level END
			WHERE tenant_id = $1 AND id::text = $2 AND state <> 'closed'
			RETURNING state`, tenantID, id, now, delta, ready, logLevel).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT state FROM mcp.sessions WHERE tenant_id = $1 AND id::text = $2`, tenantID, id).Scan(&state)
		}
		if err != nil {
			return fmt.Errorf("postgres: touch session: %w", err)
		}
		return nil
	})
	return state, unwrapNotFound(err)
}

func (r *Repository) CloseSession(ctx context.Context, tenantID, id, reason string, now time.Time, events []domain.OutboxRecord) (bool, error) {
	closed := false
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mcp.sessions SET state = 'closed', closed_at = $3, close_reason = $4
			WHERE tenant_id = $1 AND id::text = $2 AND state <> 'closed'`, tenantID, id, now, reason)
		if err != nil {
			return fmt.Errorf("postgres: close session: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		closed = true
		if _, err := tx.Exec(ctx, `DELETE FROM mcp.session_streams WHERE tenant_id = $1 AND session_id::text = $2`, tenantID, id); err != nil {
			return fmt.Errorf("postgres: drop session streams: %w", err)
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
	return closed, err
}

func (r *Repository) ListSessions(ctx context.Context, tenantID, userID string, now time.Time) ([]domain.Session, error) {
	var out []domain.Session
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+sessionColumns+`,
				(SELECT count(*) FROM mcp.session_streams st WHERE st.session_id = s.id AND st.heartbeat_at > $3)::int
			FROM mcp.sessions s
			WHERE s.tenant_id = $1 AND s.state <> 'closed' AND ($2 = '' OR s.user_id::text = $2)
			ORDER BY s.created_at DESC, s.id`, tenantID, userID, now.Add(-domain.StreamLiveWindow))
		if err != nil {
			return fmt.Errorf("postgres: list sessions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			s, err := scanSession(rows, true)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) OpenStream(ctx context.Context, tenantID, userID, sessionID, replicaID, kind, streamID string, maxUser, maxTenant int, now time.Time) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		// Serialize the count+insert per user so two replicas cannot both pass the cap.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, tenantID+"/"+userID); err != nil {
			return fmt.Errorf("postgres: stream lock: %w", err)
		}
		var owner string
		var state string
		if err := tx.QueryRow(ctx, `SELECT user_id::text, state FROM mcp.sessions WHERE tenant_id = $1 AND id::text = $2`, tenantID, sessionID).Scan(&owner, &state); err != nil {
			return unwrapNotFound(fmt.Errorf("postgres: stream session: %w", err))
		}
		if owner != userID || state == domain.SessionClosed {
			return domain.ErrNotFound()
		}
		cutoff := now.Add(-domain.StreamLiveWindow)
		var nu, nt int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE user_id::text = $2), count(*)
			FROM mcp.session_streams WHERE tenant_id = $1 AND kind = 'get' AND heartbeat_at > $3`, tenantID, userID, cutoff).Scan(&nu, &nt); err != nil {
			return fmt.Errorf("postgres: count streams: %w", err)
		}
		if kind == "get" && ((maxUser > 0 && nu >= maxUser) || (maxTenant > 0 && nt >= maxTenant)) {
			return domain.ErrStreamLimit()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.session_streams (id, session_id, tenant_id, user_id, replica_id, kind, opened_at, heartbeat_at)
			VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $7)`, streamID, sessionID, tenantID, userID, replicaID, kind, now); err != nil {
			return fmt.Errorf("postgres: insert stream: %w", err)
		}
		return nil
	})
}

func (r *Repository) HeartbeatStream(ctx context.Context, tenantID, streamID string, now time.Time) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE mcp.session_streams SET heartbeat_at = $3 WHERE tenant_id = $1 AND id::text = $2`, tenantID, streamID, now)
		return err
	})
}

func (r *Repository) CloseStream(ctx context.Context, tenantID, streamID string) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM mcp.session_streams WHERE tenant_id = $1 AND id::text = $2`, tenantID, streamID)
		return err
	})
}

func (r *Repository) ReapIdle(ctx context.Context, cutoff time.Time, limit int, now time.Time, mkEvent func(domain.ClosedSession) (domain.OutboxRecord, error)) ([]domain.ClosedSession, error) {
	var out []domain.ClosedSession
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE mcp.sessions SET state = 'closed', closed_at = $2, close_reason = 'idle'
			WHERE id IN (SELECT id FROM mcp.sessions WHERE state <> 'closed' AND last_seen_at < $1
				ORDER BY last_seen_at LIMIT $3 FOR UPDATE SKIP LOCKED)
			RETURNING id::text, tenant_id::text, user_id::text`, cutoff, now, limit)
		if err != nil {
			return fmt.Errorf("postgres: reap idle sessions: %w", err)
		}
		for rows.Next() {
			var c domain.ClosedSession
			if err := rows.Scan(&c.ID, &c.TenantID, &c.UserID); err != nil {
				rows.Close()
				return err
			}
			out = append(out, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, c := range out {
			ev, err := mkEvent(c)
			if err != nil {
				return err
			}
			// Outbox rows are tenant-scoped by RLS: switch the tenant GUC per row.
			if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, c.TenantID); err != nil {
				return fmt.Errorf("postgres: reap set tenant: %w", err)
			}
			if err := insertOutboxTx(ctx, tx, c.TenantID, ev); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM mcp.session_streams WHERE heartbeat_at < $1`, now.Add(-10*domain.StreamLiveWindow)); err != nil {
			return fmt.Errorf("postgres: sweep streams: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CountOpenSessions counts non-closed sessions across tenants for the
// orca_mcp_sessions_active gauge. Read-only; uses the same cross-tenant relay
// switch as ReapIdle (the sessions policy allows SELECT under it).
func (r *Repository) CountOpenSessions(ctx context.Context) (int64, error) {
	var n int64
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mcp.sessions WHERE state <> 'closed'`).Scan(&n); err != nil {
			return fmt.Errorf("postgres: count open sessions: %w", err)
		}
		return nil
	})
	return n, err
}
