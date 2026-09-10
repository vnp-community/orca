package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// sessionExpiry — BR-AG-08.
const sessionExpiry = 7 * 24 * time.Hour

type ResumeAgentSessionInput struct {
	ConnectionID string
	WorktreeID   string
	UserID       string
	Cwd          string
	Cols, Rows   int32
}

// ResumeAgentSession loads the latest AgentSession for a worktree,
// validates BR-AG-08/BR-AG-09, then delegates to StartAgentSession with
// ResumeID populated — a thin composition, not a fork of the spawn logic.
type ResumeAgentSession struct {
	sessions AgentSessionRepository
	resolver ConnectionResolver
	start    *StartAgentSession
	clock    func() time.Time
}

func NewResumeAgentSession(sessions AgentSessionRepository, resolver ConnectionResolver, start *StartAgentSession) *ResumeAgentSession {
	return &ResumeAgentSession{sessions: sessions, resolver: resolver, start: start, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *ResumeAgentSession) Execute(ctx context.Context, in ResumeAgentSessionInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, prior, err := uc.sessions.LatestForWorktree(ctx, tenantID, in.WorktreeID)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_RESUME_LOOKUP_FAILED", "failed to load prior agent session", err)
	}
	if !found {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "no prior session for this worktree", nil)
	}

	// BR-AG-08: 7-day inactivity expiry, measured from LastActiveAt (not
	// StartedAt) — a long-lived-but-recently-touched session should not
	// expire just because it started a while ago.
	if uc.clock().Sub(prior.LastActiveAt) > sessionExpiry {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_SESSION_EXPIRED", "session expired — start a new session", domain.ErrAgentSessionExpired)
	}
	if prior.ResumeProviderSessionID == "" {
		// The agent.hook capture path (TASK-AG-03-05) never reported a
		// provider session id for this run — most likely killed before the
		// CLI reported one. Honest failure, not a silent fresh-start
		// substitution — resume-vs-fresh-start is an explicit user decision.
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_NO_RESUMABLE_SESSION", "no resumable provider session id was captured for the prior run", nil)
	}

	// BR-AG-09: resume must use the same agent version as the original
	// session. devServer.AgentVersion is this connection's CURRENT agent
	// build; compared against what was stored at spawn time.
	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err == nil && connected && prior.AgentVersion != "" && devServer.AgentVersion != prior.AgentVersion {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_VERSION_MISMATCH", "agent version differs from the original session — start a new session?", domain.ErrAgentVersionMismatch)
	}

	return uc.start.Execute(ctx, StartAgentSessionInput{
		ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID,
		Cwd: in.Cwd, ModelID: prior.ModelID, AccountID: prior.AccountID, Cols: in.Cols, Rows: in.Rows,
		ResumeID: prior.ResumeProviderSessionID, // the CLI's own id, not prior.ID
	})
}
