package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// AgentSessionStore implements usecase.AgentSessionRepository against
// infra.agent_sessions (migrations/0007_agent_sessions, extended by
// migrations/0008_agent_sessions_resume) — split into its own type over the
// shared pool, same rationale as TerminalSessionStore.
type AgentSessionStore struct {
	pool *pgxpool.Pool
}

func NewAgentSessionStore(pool *pgxpool.Pool) *AgentSessionStore {
	return &AgentSessionStore{pool: pool}
}

// pgUniqueViolation is Postgres's "unique_violation" SQLSTATE — used to
// detect BR-AG-01's partial unique index rejecting a concurrent Create,
// distinct from any other insert failure.
const pgUniqueViolation = "23505"

func (s *AgentSessionStore) Create(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	if session.ID == "" {
		return domain.AgentSession{}, fmt.Errorf("postgres: agent session id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.agent_sessions
			(id, tenant_id, pty_id, connection_id, worktree_id, dev_server_id, user_id, model_id, account_id,
			 resume_of_session_id, agent_version, status, started_at, last_active_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, $6, $7, $8, NULLIF($9, '')::uuid,
		        NULLIF($10, '')::uuid, NULLIF($11, ''), $12, $13, $14)
	`, session.ID, session.TenantID, session.PtyID, session.ConnectionID, session.WorktreeID, session.DevServerID,
		session.UserID, session.ModelID, session.AccountID,
		session.ResumeOfSessionID, session.AgentVersion, string(session.Status),
		session.StartedAt, session.LastActiveAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.AgentSession{}, domain.ErrAgentAlreadyRunning
		}
		return domain.AgentSession{}, fmt.Errorf("postgres: insert agent session: %w", err)
	}
	return session, nil
}

func (s *AgentSessionStore) Get(ctx context.Context, tenantID, sessionID string) (bool, domain.AgentSession, error) {
	row := s.pool.QueryRow(ctx, agentSessionSelect+`WHERE tenant_id = $1 AND id = $2`, tenantID, sessionID)
	session, err := scanAgentSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("postgres: query agent session: %w", err)
	}
	return true, session, nil
}

// GetByPtyID — TASK-AG-03-07's exact join key for agent.hook correlation.
func (s *AgentSessionStore) GetByPtyID(ctx context.Context, tenantID, ptyID string) (bool, domain.AgentSession, error) {
	row := s.pool.QueryRow(ctx, agentSessionSelect+`WHERE tenant_id = $1 AND pty_id = $2`, tenantID, ptyID)
	session, err := scanAgentSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("postgres: query agent session by pty_id: %w", err)
	}
	return true, session, nil
}

func (s *AgentSessionStore) LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	row := s.pool.QueryRow(ctx,
		agentSessionSelect+`WHERE tenant_id = $1 AND worktree_id = $2 ORDER BY started_at DESC LIMIT 1`,
		tenantID, worktreeID)
	session, err := scanAgentSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("postgres: query latest agent session: %w", err)
	}
	return true, session, nil
}

// MostRecentActiveForWorktree — the agent.hook correlation fallback
// (TASK-AG-03-05's "genuine gap" option 2): most recent AgentSession in
// spawning/running/idle/waiting status for worktreeID.
func (s *AgentSessionStore) MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	row := s.pool.QueryRow(ctx,
		agentSessionSelect+`WHERE tenant_id = $1 AND worktree_id = $2
		                     AND status IN ('spawning','running','idle','waiting')
		                     ORDER BY started_at DESC LIMIT 1`,
		tenantID, worktreeID)
	session, err := scanAgentSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("postgres: query most recent active agent session: %w", err)
	}
	return true, session, nil
}

func (s *AgentSessionStore) UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions SET status = $1, last_active_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, string(status), now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: update agent session status: %w", err)
	}
	return nil
}

func (s *AgentSessionStore) MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions SET status = 'stopped', stopped_at = $1, last_active_at = $1
		WHERE tenant_id = $2 AND id = $3
	`, now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: mark agent session stopped: %w", err)
	}
	return nil
}

// MarkStoppedWithStatus is MarkStopped's exit-driven counterpart — sets a
// terminal status ('stopped' or 'error', decided by the caller from the
// pty's exit code) rather than always 'stopped'.
func (s *AgentSessionStore) MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions SET status = $1, stopped_at = $2, last_active_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, string(status), now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: mark agent session stopped with status: %w", err)
	}
	return nil
}

// UpdateProviderSession persists the CLI's own resumable session id,
// captured from an agent.hook notification.
func (s *AgentSessionStore) UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions
		SET resume_provider_session_key = $1, resume_provider_session_id = $2
		WHERE tenant_id = $3 AND id = $4
	`, providerSessionKey, providerSessionID, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: update agent session provider session: %w", err)
	}
	return nil
}

const agentSessionSelect = `
	SELECT id, tenant_id, pty_id, COALESCE(connection_id::text, ''), worktree_id, dev_server_id, user_id, model_id,
	       COALESCE(account_id::text, ''), COALESCE(resume_of_session_id::text, ''),
	       COALESCE(agent_version, ''), status, started_at, last_active_at, stopped_at,
	       COALESCE(resume_provider_session_key, ''), COALESCE(resume_provider_session_id, '')
	FROM infra.agent_sessions
`

func scanAgentSession(row pgx.Row) (domain.AgentSession, error) {
	var s domain.AgentSession
	var status string
	var stoppedAt *time.Time
	if err := row.Scan(&s.ID, &s.TenantID, &s.PtyID, &s.ConnectionID, &s.WorktreeID, &s.DevServerID, &s.UserID,
		&s.ModelID, &s.AccountID, &s.ResumeOfSessionID, &s.AgentVersion, &status,
		&s.StartedAt, &s.LastActiveAt, &stoppedAt, &s.ResumeProviderSessionKey, &s.ResumeProviderSessionID); err != nil {
		return domain.AgentSession{}, err
	}
	s.Status = domain.AgentStatus(status)
	s.StoppedAt = stoppedAt
	return s, nil
}
