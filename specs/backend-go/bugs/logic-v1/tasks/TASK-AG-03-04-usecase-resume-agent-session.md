# TASK-AG-03-04: `ResumeAgentSession` usecase

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/resume_agent_session.go` (new)
**Depends on:** TASK-AG-01-07, TASK-AG-03-02
**Status:** `[ ]` TODO

---

## Context

`ResumeAgentSession` is a thin composition over `StartAgentSession`: load the latest `AgentSession` for the worktree, validate BR-AG-08 (7-day inactivity expiry) and BR-AG-09 (agent version match), then call `StartAgentSession.Execute` with `ResumeID` set to the CLI's own captured provider-session id — **not** `AgentSession.ID`.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/resume_agent_session.go`:

```go
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
```

Note: `domain.DevServer` needs an `AgentVersion` field for the
`devServer.AgentVersion != prior.AgentVersion` comparison to compile — check
`internal/domain/dev_server.go`; if the field doesn't exist yet, add it
(`AgentVersion string`) as part of this task, sourced from whatever the
agent's handshake/health-check already reports (see
`infra-fleet-service.md`'s `dev_servers.agent_version` column reference) —
if no such reporting path exists yet, leave the comparison guarded by
`prior.AgentVersion != ""` as written (an empty stored value skips the
check rather than false-positiving) and flag the missing handshake field as
a follow-up, not a blocker for this task.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestResumeAgentSession -v
```

Add `resume_agent_session_test.go`:
- no prior session → `INFRA_AGENT_SESSION_NOT_FOUND`.
- `LastActiveAt` > 7 days ago → `INFRA_AGENT_SESSION_EXPIRED`, `start.Execute` never called.
- no `ResumeProviderSessionID` captured → `INFRA_AGENT_NO_RESUMABLE_SESSION`, distinct code from expiry.
- `devServer.AgentVersion` mismatch → `INFRA_AGENT_VERSION_MISMATCH`.
- happy path → asserts `start.Execute` called with `ResumeID = prior.ResumeProviderSessionID`, **not** `prior.ID` (regression guard against id confusion).
