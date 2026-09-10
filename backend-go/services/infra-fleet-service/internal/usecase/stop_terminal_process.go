package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// signalInterrupt is the POSIX signal StopTerminalProcess sends — a real
// pty.sendSignal call (see DevServerAgentClient.SendSignal's doc comment),
// not the former WritePty(0x03)-Ctrl-C-byte workaround this replaces.
// SIGINT is the right default for "interrupt the foreground process without
// tearing the session down" (StopTerminalProcessRequest's proto doc
// comment); SIGTERM/SIGKILL are reachable through the same port method but
// aren't exposed by this RPC, which is scoped to "stop", not "kill" (see
// KillTerminalSession/KillPty for teardown).
const signalInterrupt = "SIGINT"

// StopTerminalProcess sends SIGINT to ptyID's foreground process without
// tearing the session down.
type StopTerminalProcess struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewStopTerminalProcess(sessions TerminalSessionRepository, resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *StopTerminalProcess {
	return &StopTerminalProcess{sessions: sessions, resolver: resolver, devServers: devServers, agent: agent}
}

func (uc *StopTerminalProcess) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver, uc.devServers)
	if err != nil {
		return err
	}

	if err := uc.agent.SendSignal(ctx, devServer, ptyID, signalInterrupt); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STOP_PROCESS_FAILED", "failed to send interrupt to pty's foreground process", err)
	}
	return nil
}
