package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// SessionRepository persists mcp.sessions and mcp.session_streams.
type SessionRepository interface {
	CreateSession(ctx context.Context, s domain.Session) (domain.Session, error)
	// GetSessionBySecretHash returns closed sessions too (State=closed).
	GetSessionBySecretHash(ctx context.Context, tenantID string, hash []byte) (domain.Session, error)
	GetSession(ctx context.Context, tenantID, id string) (domain.Session, error)
	// TouchSession bumps last_seen_at (open sessions only) and returns the state.
	TouchSession(ctx context.Context, tenantID, id string, ready bool, toolCallsDelta int64, logLevel string, now time.Time) (string, error)
	// CloseSession closes an open session and enqueues the events in the same
	// transaction. closed=false means it was already closed (idempotent).
	CloseSession(ctx context.Context, tenantID, id, reason string, now time.Time, events []domain.OutboxRecord) (closed bool, err error)
	// ListSessions lists open sessions; userID=="" means the whole tenant.
	ListSessions(ctx context.Context, tenantID, userID string, now time.Time) ([]domain.Session, error)
	OpenStream(ctx context.Context, tenantID, userID, sessionID, replicaID, kind, streamID string, maxUser, maxTenant int, now time.Time) error
	HeartbeatStream(ctx context.Context, tenantID, streamID string, now time.Time) error
	CloseStream(ctx context.Context, tenantID, streamID string) error
	// ReapIdle closes sessions idle since before cutoff across tenants, enqueuing
	// one session.closed event per row built by mkEvent, and sweeps stale streams.
	ReapIdle(ctx context.Context, cutoff time.Time, limit int, now time.Time, mkEvent func(domain.ClosedSession) (domain.OutboxRecord, error)) ([]domain.ClosedSession, error)
}
