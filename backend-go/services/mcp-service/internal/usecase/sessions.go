package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const maxClientField = 200

// Sessions is the use-case group of BE-MCP-SOL-004 (sessions + stream registry).
type Sessions struct {
	repo  SessionRepository
	clock Clock
}

func NewSessions(repo SessionRepository, clock Clock) *Sessions {
	return &Sessions{repo: repo, clock: clock}
}

type CreateSessionInput struct {
	SecretHash                                                      []byte
	ClientID, ClientName, ClientVersion, GrantID, TokenID, ProtoVer string
	CapabilitiesJSON                                                []byte
}

func clip(s string) string {
	if len(s) > maxClientField {
		return s[:maxClientField]
	}
	return s
}

func (uc *Sessions) Create(ctx context.Context, in CreateSessionInput) (domain.Session, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	if len(in.SecretHash) != 32 || in.ProtoVer == "" {
		return domain.Session{}, domain.ErrInvalidArgument("secret_hash (SHA-256) and protocol_version are required")
	}
	caps := in.CapabilitiesJSON
	if len(caps) == 0 {
		caps = []byte("{}")
	}
	now := uc.clock.Now()
	s, err := uc.repo.CreateSession(ctx, domain.Session{
		ID: uuid.NewString(), TenantID: id.TenantID, UserID: id.UserID, SecretHash: in.SecretHash,
		ClientID: clip(in.ClientID), ClientName: clip(in.ClientName), ClientVersion: clip(in.ClientVersion),
		GrantID: in.GrantID, TokenID: in.TokenID, ProtocolVersion: in.ProtoVer, CapabilitiesJSON: caps,
		LogLevel: "warning", State: domain.SessionInitializing, CreatedAt: now, LastSeenAt: now,
	})
	if err != nil {
		return domain.Session{}, wrapRepoErr(err, "failed to create session")
	}
	return s, nil
}

// GetBySecret is tenant-scoped (RLS); the gateway compares the owner itself so
// it can count identity mismatches.
func (uc *Sessions) GetBySecret(ctx context.Context, hash []byte) (domain.Session, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	s, err := uc.repo.GetSessionBySecretHash(ctx, id.TenantID, hash)
	if err != nil {
		return domain.Session{}, wrapRepoErr(err, "failed to load session")
	}
	return s, nil
}

func (uc *Sessions) Touch(ctx context.Context, sessionID string, ready bool, delta int64, logLevel string) (string, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return "", err
	}
	if delta < 0 {
		delta = 0
	}
	st, err := uc.repo.TouchSession(ctx, id.TenantID, sessionID, ready, delta, logLevel, uc.clock.Now())
	if err != nil {
		return "", wrapRepoErr(err, "failed to touch session")
	}
	return st, nil
}

type CloseSessionInput struct {
	SessionID string
	Hash      []byte
	Reason    string
}

// Close: a plain user may only close their own session; admins any in the
// tenant. Foreign or unknown sessions are indistinguishable (MCP_NOT_FOUND).
// Closing an already closed session is a no-op success.
func (uc *Sessions) Close(ctx context.Context, in CloseSessionInput) (string, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return "", err
	}
	if !domain.ValidCloseReason(in.Reason) {
		return "", domain.ErrInvalidArgument("unknown close reason")
	}
	var s domain.Session
	if in.SessionID != "" {
		s, err = uc.repo.GetSession(ctx, id.TenantID, in.SessionID)
	} else {
		s, err = uc.repo.GetSessionBySecretHash(ctx, id.TenantID, in.Hash)
	}
	if err != nil {
		return "", wrapRepoErr(err, "failed to load session")
	}
	if s.UserID != id.UserID && id.Role != domain.RoleAdmin {
		return "", domain.ErrNotFound()
	}
	if s.State == domain.SessionClosed {
		return s.ID, nil
	}
	now := uc.clock.Now()
	ev, err := closedEvent(domain.ClosedSession{ID: s.ID, TenantID: s.TenantID, UserID: s.UserID}, in.Reason, now)
	if err != nil {
		return "", domain.ErrInternal("failed to build event", err)
	}
	if _, err := uc.repo.CloseSession(ctx, id.TenantID, s.ID, in.Reason, now, []domain.OutboxRecord{ev}); err != nil {
		return "", wrapRepoErr(err, "failed to close session")
	}
	return s.ID, nil
}

func closedEvent(c domain.ClosedSession, reason string, at time.Time) (domain.OutboxRecord, error) {
	return domain.NewOutboxEvent(uuid.NewString(), domain.SubjectSessionClosed, c.TenantID, at,
		map[string]any{"session_id": c.ID, "user_id": c.UserID, "reason": reason})
}

// List returns the caller's open sessions; admin=true the whole tenant.
func (uc *Sessions) List(ctx context.Context, admin bool) ([]domain.Session, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	user := id.UserID
	if admin {
		if err := id.requireAdmin(); err != nil {
			return nil, err
		}
		user = ""
	}
	out, err := uc.repo.ListSessions(ctx, id.TenantID, user, uc.clock.Now())
	if err != nil {
		return nil, wrapRepoErr(err, "failed to list sessions")
	}
	return out, nil
}

func (uc *Sessions) OpenStream(ctx context.Context, sessionID, replica, kind string, maxUser, maxTenant int) (string, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return "", err
	}
	if kind != "get" && kind != "post" {
		return "", domain.ErrInvalidArgument("kind must be get or post")
	}
	sid := uuid.NewString()
	if err := uc.repo.OpenStream(ctx, id.TenantID, id.UserID, sessionID, clip(replica), kind, sid, maxUser, maxTenant, uc.clock.Now()); err != nil {
		return "", wrapRepoErr(err, "failed to open stream")
	}
	return sid, nil
}

func (uc *Sessions) Heartbeat(ctx context.Context, streamID string) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if err := uc.repo.HeartbeatStream(ctx, id.TenantID, streamID, uc.clock.Now()); err != nil {
		return wrapRepoErr(err, "failed to heartbeat stream")
	}
	return nil
}

func (uc *Sessions) CloseStream(ctx context.Context, streamID string) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if err := uc.repo.CloseStream(ctx, id.TenantID, streamID); err != nil {
		return wrapRepoErr(err, "failed to close stream")
	}
	return nil
}

// ReapIdleSessions closes sessions idle longer than ttl (all tenants); safe to
// run on every replica at once (conditional UPDATE).
type ReapIdleSessions struct {
	repo  SessionRepository
	clock Clock
	ttl   time.Duration
}

func NewReapIdleSessions(repo SessionRepository, clock Clock, ttl time.Duration) *ReapIdleSessions {
	return &ReapIdleSessions{repo: repo, clock: clock, ttl: ttl}
}

func (uc *ReapIdleSessions) Execute(ctx context.Context, limit int) (int, error) {
	now := uc.clock.Now()
	rows, err := uc.repo.ReapIdle(ctx, now.Add(-uc.ttl), limit, now, func(c domain.ClosedSession) (domain.OutboxRecord, error) {
		return closedEvent(c, domain.CloseReasonIdle, now)
	})
	return len(rows), err
}
