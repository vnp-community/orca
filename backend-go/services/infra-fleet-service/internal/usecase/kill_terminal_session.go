package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TerminalSessionCloser marks a session closed AND enqueues event in ONE
// database transaction, so the terminal.closed event exists iff the close
// committed. transitioned is false (and nothing is enqueued) when the session
// was already closed; a missing session is an error.
type TerminalSessionCloser interface {
	CloseWithEvent(ctx context.Context, tenantID, ptyID string, closedAt time.Time, event domain.OutboxEvent) (transitioned bool, err error)
}

// KillTerminalSessionInput carries the optional audit context of a close.
type KillTerminalSessionInput struct {
	PtyID  string
	Reason string // "user" | "idle" | "session_closed"; anything else is "user"
	Actor  string // caller's user id when it differs from the session owner
}

// KillTerminalSession backs terminal.close — full teardown (pty.destroy),
// distinct from StopTerminalProcess's foreground-interrupt-only contract.
// Marks the session row closed even if the agent call fails, so a dev
// server that has already gone away (or a pty the agent no longer knows
// about) doesn't leave a permanently "open" row behind — mirrors this
// codebase's general "the persisted record must not lie" discipline.
type KillTerminalSession struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
	closer     TerminalSessionCloser // nil: plain Close, no event
}

func NewKillTerminalSession(sessions TerminalSessionRepository, resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *KillTerminalSession {
	return &KillTerminalSession{sessions: sessions, resolver: resolver, devServers: devServers, agent: agent}
}

// WithClosedEvents makes the close enqueue orca.infrafleet.terminal.closed
// atomically with the state change.
func (uc *KillTerminalSession) WithClosedEvents(closer TerminalSessionCloser) *KillTerminalSession {
	uc.closer = closer
	return uc
}

func (uc *KillTerminalSession) Execute(ctx context.Context, ptyID string) error {
	return uc.ExecuteWithInput(ctx, KillTerminalSessionInput{PtyID: ptyID})
}

func (uc *KillTerminalSession) ExecuteWithInput(ctx context.Context, in KillTerminalSessionInput) error {
	ptyID := in.PtyID
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	session, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver, uc.devServers)
	if err != nil {
		return err
	}

	agentErr := uc.agent.KillPty(ctx, devServer, ptyID, true)

	if err := uc.markClosed(ctx, tenantID, session, in); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_CLOSE_TERMINAL_SESSION_FAILED", "failed to mark terminal session closed", err)
	}
	if agentErr != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_KILL_PTY_FAILED", "terminal session marked closed, but the dev server agent failed to tear down the pty", agentErr)
	}
	return nil
}

func (uc *KillTerminalSession) markClosed(ctx context.Context, tenantID string, session domain.TerminalSession, in KillTerminalSessionInput) error {
	now := time.Now().UTC()
	if uc.closer == nil {
		return uc.sessions.Close(ctx, tenantID, in.PtyID, now)
	}
	event, err := domain.NewTerminalClosedEvent(session, in.Reason, in.Actor, now)
	if err != nil {
		return err
	}
	_, err = uc.closer.CloseWithEvent(ctx, tenantID, in.PtyID, now, event)
	return err
}
