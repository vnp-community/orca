package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// SpawnTerminalSessionInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type SpawnTerminalSessionInput struct {
	ConnectionID string
	Cwd          string
	Shell        string
	Cols         int32
	Rows         int32
	// Command, when set, is the initial command line the spawned shell runs
	// instead of an interactive prompt — see infrafleet.proto's
	// SpawnTerminalSessionRequest.command doc comment (TASK-INT-01-01).
	Command string
	// UserID engages pty-handler.ts's per-user GH_CONFIG_DIR/GLAB_CONFIG_DIR
	// isolation for a gh/glab Command — always the caller's authenticated
	// identity, set server-side by wscompat, never client-supplied.
	UserID string
}

// SpawnTerminalSession creates a new PTY on the dev server ConnectionID
// resolves to (via pty.create, see DevServerAgentClient.SpawnPty) and
// persists the resulting session in TerminalSessionRepository — the write
// path every other terminal usecase's resolveTerminalSession lookup reads
// from.
//
// Host-local sessions (ConnectionID == ""): the proto's doc comment says
// these are "rejected in server-deployment mode" — serverDeployment enforces
// exactly that. Outside server-deployment mode this service STILL cannot
// spawn a host-local PTY itself: there is no local-pty adapter in
// backend-go (PTYs only exist inside the agent's detached pty-daemon
// process, see adapter/devserveragent's package doc comment) — so a
// host-local request always fails today, with a distinct error explaining
// why, rather than silently no-opping. Tracked as a known gap, not
// implemented by this pass.
type SpawnTerminalSession struct {
	resolver         ConnectionResolver
	agent            DevServerAgentClient
	sessions         TerminalSessionRepository
	serverDeployment bool
}

func NewSpawnTerminalSession(resolver ConnectionResolver, agent DevServerAgentClient, sessions TerminalSessionRepository, serverDeployment bool) *SpawnTerminalSession {
	return &SpawnTerminalSession{resolver: resolver, agent: agent, sessions: sessions, serverDeployment: serverDeployment}
}

func (uc *SpawnTerminalSession) Execute(ctx context.Context, in SpawnTerminalSessionInput) (domain.TerminalSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	if in.ConnectionID == "" {
		if uc.serverDeployment {
			return domain.TerminalSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_TERMINAL_HOST_LOCAL_DISABLED", "host-local terminal sessions are disabled in server-deployment mode", nil)
		}
		return domain.TerminalSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_TERMINAL_HOST_LOCAL_UNIMPLEMENTED", "host-local terminal sessions are not implemented — every PTY this service can spawn today must go through a connectionId-bound dev server agent", nil)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", nil)
	}

	result, err := uc.agent.SpawnPty(ctx, devServer, SpawnPtyInput{Cwd: in.Cwd, Shell: in.Shell, Cols: in.Cols, Rows: in.Rows, Command: in.Command, UserID: in.UserID})
	if err != nil {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SPAWN_PTY_FAILED", "failed to spawn pty on dev server agent", err)
	}
	if result.PtyID == "" {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SPAWN_PTY_NO_ID", "dev server agent did not return a pty id", nil)
	}

	now := time.Now().UTC()
	cwd := result.Cwd
	if cwd == "" {
		cwd = in.Cwd
	}
	session, err := uc.sessions.Create(ctx, domain.TerminalSession{
		PtyID:        result.PtyID,
		TenantID:     tenantID,
		ConnectionID: in.ConnectionID,
		Cwd:          cwd,
		CreatedAt:    now,
		LastActiveAt: now,
	})
	if err != nil {
		return domain.TerminalSession{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_TERMINAL_SESSION_FAILED", "failed to persist terminal session", err)
	}
	return session, nil
}
