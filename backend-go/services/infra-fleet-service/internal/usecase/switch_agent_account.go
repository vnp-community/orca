package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type SwitchAgentAccountInput struct {
	ConnectionID string
	WorktreeID   string
	UserID       string
	ProjectID    string
	Cwd          string
}

// SwitchAgentAccount — BL-AG-04's saga: force-kill the current session,
// resolve a replacement account (excluding the one just switched away
// from), then resume if BR-AG-09-compatible or start fresh otherwise.
type SwitchAgentAccount struct {
	sessions AgentSessionRepository
	kill     *KillAgentSession
	resolve  AIProviderResolverClient
	start    *StartAgentSession
	resume   *ResumeAgentSession
}

func NewSwitchAgentAccount(sessions AgentSessionRepository, kill *KillAgentSession, resolve AIProviderResolverClient, start *StartAgentSession, resume *ResumeAgentSession) *SwitchAgentAccount {
	return &SwitchAgentAccount{sessions: sessions, kill: kill, resolve: resolve, start: start, resume: resume}
}

func (uc *SwitchAgentAccount) Execute(ctx context.Context, in SwitchAgentAccountInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, current, err := uc.sessions.LatestForWorktree(ctx, tenantID, in.WorktreeID)
	if err != nil || !found {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "no running agent session for this worktree", err)
	}

	// Force kill — a switch aborts the current run entirely (BL-AG-04 step 4b).
	if err := uc.kill.Execute(ctx, current.ID, "SIGKILL"); err != nil {
		return domain.AgentSession{}, err // kill failure aborts the switch — do not spawn a second agent on top of a possibly-still-alive one
	}

	// Priority cascade, excluding the account just switched away from —
	// see TASK-AG-04-02.
	_, accountID, _, err := uc.resolve.ResolveProvider(ctx, tenantID, in.UserID, in.ProjectID, current.AccountID)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_SWITCH_RESOLVE_FAILED", "failed to resolve a replacement provider account", err)
	}
	if accountID == "" || accountID == current.AccountID {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SWITCH_NO_ALTERNATE_ACCOUNT", "provider resolution returned no usable alternate account", nil)
	}

	// Resume iff BR-AG-09 compatibility holds — delegated entirely to
	// ResumeAgentSession's own checks (expiry, resumable-id-present,
	// version match), not duplicated here. A resume attempt that fails one
	// of those checks falls back to a fresh start rather than propagating
	// the resume error, since a switch's whole point is "get an agent
	// running again," not "resume or nothing."
	if current.ResumeProviderSessionID != "" {
		resumed, err := uc.resume.Execute(ctx, ResumeAgentSessionInput{
			ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID, Cwd: in.Cwd,
		})
		if err == nil {
			return resumed, nil
		}
	}
	return uc.start.Execute(ctx, StartAgentSessionInput{
		ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID,
		Cwd: in.Cwd, ModelID: current.ModelID, AccountID: accountID,
	})
}
