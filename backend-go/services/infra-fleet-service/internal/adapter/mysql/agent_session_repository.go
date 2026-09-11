package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// AgentSessionStore implements usecase.AgentSessionRepository against
// agent_sessions (migrations/mysql/0019, extended by migrations/mysql/0028).
type AgentSessionStore struct {
	db *sql.DB
}

func NewAgentSessionStore(db *sql.DB) *AgentSessionStore {
	return &AgentSessionStore{db: db}
}

// mysqlDuplicateEntry is go-sql-driver/mysql's error number for a unique
// key violation (ER_DUP_ENTRY) — detects BR-AG-01's
// active_per_worktree_user_key generated-column unique index rejecting a
// concurrent Create (migrations/mysql/0019's translation of the Postgres
// partial unique index), the MySQL counterpart to
// internal/adapter/postgres's pgUniqueViolation ("23505").
const mysqlDuplicateEntry = 1062

func (s *AgentSessionStore) Create(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	if session.ID == "" {
		return domain.AgentSession{}, fmt.Errorf("mysql: agent session id is required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_sessions
			(id, tenant_id, pty_id, connection_id, worktree_id, dev_server_id, user_id, model_id, account_id,
			 resume_of_session_id, agent_version, status, started_at, last_active_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, NULLIF(?, ''),
		        NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
	`, session.ID, session.TenantID, session.PtyID, session.ConnectionID, session.WorktreeID, session.DevServerID,
		session.UserID, session.ModelID, session.AccountID,
		session.ResumeOfSessionID, session.AgentVersion, string(session.Status),
		session.StartedAt, session.LastActiveAt)
	if err != nil {
		var myErr *mysqldriver.MySQLError
		if errors.As(err, &myErr) && myErr.Number == mysqlDuplicateEntry {
			return domain.AgentSession{}, domain.ErrAgentAlreadyRunning
		}
		return domain.AgentSession{}, fmt.Errorf("mysql: insert agent session: %w", err)
	}
	return session, nil
}

func (s *AgentSessionStore) Get(ctx context.Context, tenantID, sessionID string) (bool, domain.AgentSession, error) {
	row := s.db.QueryRowContext(ctx, agentSessionSelect+`WHERE tenant_id = ? AND id = ?`, tenantID, sessionID)
	session, err := scanAgentSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("mysql: query agent session: %w", err)
	}
	return true, session, nil
}

// GetByPtyID — TASK-AG-03-07's exact join key for agent.hook correlation.
func (s *AgentSessionStore) GetByPtyID(ctx context.Context, tenantID, ptyID string) (bool, domain.AgentSession, error) {
	row := s.db.QueryRowContext(ctx, agentSessionSelect+`WHERE tenant_id = ? AND pty_id = ?`, tenantID, ptyID)
	session, err := scanAgentSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("mysql: query agent session by pty_id: %w", err)
	}
	return true, session, nil
}

func (s *AgentSessionStore) LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	row := s.db.QueryRowContext(ctx,
		agentSessionSelect+`WHERE tenant_id = ? AND worktree_id = ? ORDER BY started_at DESC LIMIT 1`,
		tenantID, worktreeID)
	session, err := scanAgentSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("mysql: query latest agent session: %w", err)
	}
	return true, session, nil
}

// MostRecentActiveForWorktree — the agent.hook correlation fallback
// (TASK-AG-03-05's "genuine gap" option 2): most recent AgentSession in
// spawning/running/idle/waiting status for worktreeID.
func (s *AgentSessionStore) MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	row := s.db.QueryRowContext(ctx,
		agentSessionSelect+`WHERE tenant_id = ? AND worktree_id = ?
		                     AND status IN ('spawning','running','idle','waiting')
		                     ORDER BY started_at DESC LIMIT 1`,
		tenantID, worktreeID)
	session, err := scanAgentSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("mysql: query most recent active agent session: %w", err)
	}
	return true, session, nil
}

func (s *AgentSessionStore) UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_sessions SET status = ?, last_active_at = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("mysql: update agent session status: %w", err)
	}
	return nil
}

func (s *AgentSessionStore) MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_sessions SET status = 'stopped', stopped_at = ?, last_active_at = ?
		WHERE tenant_id = ? AND id = ?
	`, now, now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("mysql: mark agent session stopped: %w", err)
	}
	return nil
}

// MarkStoppedWithStatus is MarkStopped's exit-driven counterpart — sets a
// terminal status ('stopped' or 'error', decided by the caller from the
// pty's exit code) rather than always 'stopped'.
func (s *AgentSessionStore) MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_sessions SET status = ?, stopped_at = ?, last_active_at = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), now, now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("mysql: mark agent session stopped with status: %w", err)
	}
	return nil
}

// UpdateProviderSession persists the CLI's own resumable session id,
// captured from an agent.hook notification.
func (s *AgentSessionStore) UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_sessions
		SET resume_provider_session_key = ?, resume_provider_session_id = ?
		WHERE tenant_id = ? AND id = ?
	`, providerSessionKey, providerSessionID, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("mysql: update agent session provider session: %w", err)
	}
	return nil
}

// account_id/resume_of_session_id/agent_version/resume_provider_session_*
// need no ::text cast unlike Postgres — CHAR(36)/TEXT already coerce inside
// COALESCE.
const agentSessionSelect = `
	SELECT id, tenant_id, pty_id, COALESCE(connection_id, ''), worktree_id, dev_server_id, user_id, model_id,
	       COALESCE(account_id, ''), COALESCE(resume_of_session_id, ''),
	       COALESCE(agent_version, ''), status, started_at, last_active_at, stopped_at,
	       COALESCE(resume_provider_session_key, ''), COALESCE(resume_provider_session_id, '')
	FROM agent_sessions
`

func scanAgentSession(row rowScanner) (domain.AgentSession, error) {
	var s domain.AgentSession
	var status string
	var stoppedAt sql.NullTime
	if err := row.Scan(&s.ID, &s.TenantID, &s.PtyID, &s.ConnectionID, &s.WorktreeID, &s.DevServerID, &s.UserID,
		&s.ModelID, &s.AccountID, &s.ResumeOfSessionID, &s.AgentVersion, &status,
		&s.StartedAt, &s.LastActiveAt, &stoppedAt, &s.ResumeProviderSessionKey, &s.ResumeProviderSessionID); err != nil {
		return domain.AgentSession{}, err
	}
	s.Status = domain.AgentStatus(status)
	if stoppedAt.Valid {
		t := stoppedAt.Time
		s.StoppedAt = &t
	}
	return s, nil
}
