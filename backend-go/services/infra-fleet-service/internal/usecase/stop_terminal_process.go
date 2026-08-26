package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ctrlC is the interrupt byte (0x03, ETX) a terminal driver sends on Ctrl-C —
// FLAGGED deviation: the task instructions for TASK-181 name exactly seven
// DevServerAgentClient methods to add (SpawnPty/WritePty/ResizePty/KillPty/
// StreamPty/AgentStatus/InspectProcess), with none of them a dedicated
// "send signal" primitive — even though the real agent DOES expose one
// (pty.sendSignal, confirmed in agent/src/relay/agent-rpc-dispatch.ts).
// Rather than adding an eighth port method the task list didn't ask for,
// StopTerminalProcess ("sends an interrupt signal to the pty's foreground
// process", per StopTerminalProcessRequest's proto doc comment) is
// implemented as WritePty(0x03) — a pragmatic equivalent for the common
// SIGINT case, but NOT equivalent to a real signal delivery (SIGTERM/SIGKILL
// aren't reachable this way, and 0x03 only works if the foreground process
// has a functioning tty signal handler). See this service's final report for
// this same flag.
const ctrlC = 0x03

// StopTerminalProcess sends an interrupt to ptyID's foreground process
// without tearing the session down — see ctrlC's doc comment above for the
// FLAGGED implementation detail.
type StopTerminalProcess struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewStopTerminalProcess(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *StopTerminalProcess {
	return &StopTerminalProcess{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *StopTerminalProcess) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}

	if err := uc.agent.WritePty(ctx, devServer, ptyID, []byte{ctrlC}); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STOP_PROCESS_FAILED", "failed to send interrupt to pty's foreground process", err)
	}
	return nil
}
