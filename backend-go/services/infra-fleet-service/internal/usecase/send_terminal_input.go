package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// SendTerminalInput writes directly to a pty's stdin, bypassing AttachPty's
// stream — see SendTerminalInputRequest's proto doc comment for why a
// stateless REST/CLI caller needs this sibling to terminal.send.
type SendTerminalInput struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewSendTerminalInput(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *SendTerminalInput {
	return &SendTerminalInput{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *SendTerminalInput) Execute(ctx context.Context, ptyID string, data []byte) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}

	if err := uc.agent.WritePty(ctx, devServer, ptyID, data); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_WRITE_PTY_FAILED", "failed to write to pty", err)
	}
	return nil
}
