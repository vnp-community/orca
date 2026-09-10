package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// StartAgentSessionInput mirrors the gRPC request 1:1.
type StartAgentSessionInput struct {
	ConnectionID string
	WorktreeID   string
	UserID       string
	Cwd          string
	ModelID      string
	AccountID    string
	TrustPreset  string
	ResumeID     string // "" for a fresh start; set by ResumeAgentSession (TASK-AG-03-04)
	Cols, Rows   int32
}

// StartAgentSession spawns an AI-CLI agent via DevServerAgentClient.SpawnAgent
// and persists an AgentSession — follows SpawnTerminalSession.Execute's
// resolve->spawn->persist shape, with BR-AG-01's single-agent guard added.
type StartAgentSession struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
	sessions AgentSessionRepository

	// classifier — TASK-AG-05-06: started as a goroutine right after a new
	// session persists, one per live AgentSession. Nil-safe (skipped) so
	// every pre-existing test call site that has no use for classification
	// doesn't need to construct one.
	classifier *AgentOutputClassifier

	// ensureHookConsumer — TASK-AG-03-06: called right after
	// ResolveConnection succeeds (both StartAgentSession's own fresh-spawn
	// path AND ResumeAgentSession's delegated call go through here, so one
	// call site covers both) to lazily start RecordAgentHookProviderSession.Run
	// for devServer, idempotently — see main.go's ensureAgentHookConsumer
	// closure for the per-dev-server-id dedup guard. Nil-safe (skipped) for
	// the same reason as classifier above.
	ensureHookConsumer func(ctx context.Context, tenantID string, devServer domain.DevServer)

	clock func() time.Time
}

func NewStartAgentSession(resolver ConnectionResolver, agent DevServerAgentClient, sessions AgentSessionRepository, classifier *AgentOutputClassifier, ensureHookConsumer func(ctx context.Context, tenantID string, devServer domain.DevServer)) *StartAgentSession {
	return &StartAgentSession{
		resolver: resolver, agent: agent, sessions: sessions,
		classifier: classifier, ensureHookConsumer: ensureHookConsumer,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (uc *StartAgentSession) Execute(ctx context.Context, in StartAgentSessionInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil || !connected {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", err)
	}

	// TASK-AG-03-06: lazily start (or no-op if already running) the
	// per-dev-server agent.hook consumer — every path that needs one
	// (fresh spawn, resume) already resolves devServer right here.
	if uc.ensureHookConsumer != nil {
		uc.ensureHookConsumer(ctx, tenantID, devServer)
	}

	// Minted here, passed as SpawnAgentInput.TaskID — agent.spawn's ptyId
	// embeds this, so the session<->pty linkage is derivable even before
	// the agent's response comes back.
	sessionID := uuid.NewString()

	result, err := uc.agent.SpawnAgent(ctx, devServer, SpawnAgentInput{
		TaskID: sessionID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		Cwd: in.Cwd, WorktreePath: in.Cwd, ResumeID: in.ResumeID, Cols: in.Cols, Rows: in.Rows, TrustPreset: in.TrustPreset,
	})
	if err != nil {
		return domain.AgentSession{}, translateAgentSpawnError(err)
	}

	now := uc.clock()
	session, err := uc.sessions.Create(ctx, domain.AgentSession{
		ID: sessionID, PtyID: result.PtyID, TenantID: tenantID, ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID,
		DevServerID: devServer.ID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		AgentVersion: devServer.AgentVersion,
		Status:       domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAgentAlreadyRunning) {
			// The agent process is now orphaned on the dev server — kill it
			// rather than leave an untracked PTY running.
			_ = uc.agent.KillAgent(ctx, devServer, result.PtyID, "SIGKILL")
			return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_ALREADY_RUNNING", "an agent is already running for this worktree and user", err)
		}
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_AGENT_SESSION_FAILED", "failed to persist agent session", err)
	}

	// TASK-AG-05-06: classify this session's PTY output for real-time
	// agent.statusChanged/agent:rateLimited delivery. context.Background(),
	// not ctx — this goroutine must outlive the Start/Resume RPC call that
	// spawned it, for the session's entire lifetime.
	if uc.classifier != nil {
		tenantIDForClassifier := tenantID // avoid capturing the outer var name ambiguity in the closure below
		go uc.classifier.Run(context.Background(), tenantIDForClassifier, session, devServer)
	}

	return session, nil
}

// translateAgentSpawnError maps agent.spawn's own credential-injection
// failure messages (agent-spawner.ts's buildAgentEnv, fixed strings) into a
// dedicated apperrors kind — an honest, distinguishable error rather than a
// generic internal failure, until the agent-side Vault Transit work
// (TASK-AG-01-04) lands.
func translateAgentSpawnError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no plaintext resolvedApiKey was provided") ||
		strings.Contains(msg, "no credential found for accountId=") {
		return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE",
			"credential injection for this provider account is not available yet — Dev Server Agent has no Vault Transit decrypt (see TASK-AG-01-04)", err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SPAWN_AGENT_FAILED", "failed to spawn agent on dev server agent", err)
}
