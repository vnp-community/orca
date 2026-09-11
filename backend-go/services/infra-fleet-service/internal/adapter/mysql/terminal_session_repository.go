package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TerminalSessionStore implements usecase.TerminalSessionRepository against
// terminal_sessions (migrations/mysql/0005) — split into its own type over
// the same *sql.DB, mirroring internal/adapter/postgres.TerminalSessionStore.
type TerminalSessionStore struct {
	db *sql.DB
}

func NewTerminalSessionStore(db *sql.DB) *TerminalSessionStore {
	return &TerminalSessionStore{db: db}
}

// Create inserts a new terminal session row. connection_id/created_by_user_id
// are stored as NULL when empty (host-local sessions — see
// domain.TerminalSession's doc comment), no ::uuid cast needed since
// CHAR(36)/VARCHAR(255) already accept a plain string.
func (s *TerminalSessionStore) Create(ctx context.Context, session domain.TerminalSession) (domain.TerminalSession, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO terminal_sessions (pty_id, tenant_id, connection_id, cwd, created_at, last_active_at, created_by_user_id)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''))
	`, session.PtyID, session.TenantID, session.ConnectionID, session.Cwd, session.CreatedAt, session.LastActiveAt, session.CreatedByUserID)
	if err != nil {
		return domain.TerminalSession{}, fmt.Errorf("mysql: insert terminal session: %w", err)
	}
	return session, nil
}

// Get fetches one session scoped to tenantID — found=false (not an error)
// when no row matches, per usecase.TerminalSessionRepository's doc comment.
func (s *TerminalSessionStore) Get(ctx context.Context, tenantID, ptyID string) (bool, domain.TerminalSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT pty_id, tenant_id, connection_id, cwd, created_at, last_active_at, closed_at, created_by_user_id
		FROM terminal_sessions
		WHERE tenant_id = ? AND pty_id = ?
	`, tenantID, ptyID)

	session, err := scanTerminalSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.TerminalSession{}, nil
	}
	if err != nil {
		return false, domain.TerminalSession{}, fmt.Errorf("mysql: query terminal session: %w", err)
	}
	return true, session, nil
}

// List returns every OPEN session for tenantID, optionally narrowed to
// connectionID. connectionID is bound twice — MySQL placeholders are
// positional, unlike Postgres's $2 reused twice in one statement.
func (s *TerminalSessionStore) List(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pty_id, tenant_id, connection_id, cwd, created_at, last_active_at, closed_at, created_by_user_id
		FROM terminal_sessions
		WHERE tenant_id = ?
		  AND closed_at IS NULL
		  AND (? = '' OR connection_id = ?)
		ORDER BY created_at DESC
	`, tenantID, connectionID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query terminal sessions: %w", err)
	}
	defer rows.Close()

	var out []domain.TerminalSession
	for rows.Next() {
		session, err := scanTerminalSession(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan terminal session row: %w", err)
		}
		out = append(out, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate terminal session rows: %w", err)
	}
	return out, nil
}

// Touch bumps last_active_at for one session scoped to tenantID.
// last_active_at always transitions to a fresh time.Now() value, never a
// same-value retry, so RowsAffected() == 0 unambiguously means not-found —
// same reasoning as AgentTokenStore.Revoke, no BE-DB-SOL-005 §3.1 pitfall.
func (s *TerminalSessionStore) Touch(ctx context.Context, tenantID, ptyID string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE terminal_sessions SET last_active_at = ?
		WHERE tenant_id = ? AND pty_id = ?
	`, now, tenantID, ptyID)
	if err != nil {
		return fmt.Errorf("mysql: touch terminal session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mysql: terminal session %q not found for tenant", ptyID)
	}
	return nil
}

// Close sets closed_at for one session scoped to tenantID — idempotent, see
// usecase.TerminalSessionRepository.Close's doc comment. A second Close call
// with a newer closedAt still changes the column's value each time (a
// fresh time.Now()), so — same as Touch — RowsAffected() == 0 unambiguously
// means "no such session for this tenant", never a same-value no-op.
func (s *TerminalSessionStore) Close(ctx context.Context, tenantID, ptyID string, closedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE terminal_sessions SET closed_at = ?
		WHERE tenant_id = ? AND pty_id = ?
	`, closedAt, tenantID, ptyID)
	if err != nil {
		return fmt.Errorf("mysql: close terminal session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mysql: terminal session %q not found for tenant", ptyID)
	}
	return nil
}

// CloseAllForConnection sets closed_at for every OPEN session bound to
// connectionID — BE-SOL-STORAGE-003 §3/TASK-BE-STORAGE-010. Zero matching
// rows is not an error — a connection with no open terminal sessions is a
// normal, expected case (RowsAffected() is never even inspected here,
// matching the Postgres variant).
func (s *TerminalSessionStore) CloseAllForConnection(ctx context.Context, tenantID, connectionID string, closedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE terminal_sessions
		SET closed_at = ?
		WHERE tenant_id = ? AND connection_id = ? AND closed_at IS NULL
	`, closedAt, tenantID, connectionID)
	if err != nil {
		return fmt.Errorf("mysql: close all terminal sessions for connection: %w", err)
	}
	return nil
}

func scanTerminalSession(row rowScanner) (domain.TerminalSession, error) {
	var session domain.TerminalSession
	var connectionID sql.NullString
	var closedAt sql.NullTime
	var createdByUserID sql.NullString
	err := row.Scan(&session.PtyID, &session.TenantID, &connectionID, &session.Cwd, &session.CreatedAt, &session.LastActiveAt, &closedAt, &createdByUserID)
	if err != nil {
		return domain.TerminalSession{}, err
	}
	if connectionID.Valid {
		session.ConnectionID = connectionID.String
	}
	if closedAt.Valid {
		t := closedAt.Time
		session.ClosedAt = &t
	}
	if createdByUserID.Valid {
		session.CreatedByUserID = createdByUserID.String
	}
	return session, nil
}
