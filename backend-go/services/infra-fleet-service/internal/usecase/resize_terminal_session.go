package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ResizeTerminalSessionInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type ResizeTerminalSessionInput struct {
	PtyID string
	Cols  int32
	Rows  int32
}

// ResizeTerminalSession is the unary counterpart to AttachPty's in-stream
// PtyResize frame (see PtyResizeMessage's doc comment) — same underlying
// agent call, reachable without an open stream.
type ResizeTerminalSession struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewResizeTerminalSession(sessions TerminalSessionRepository, resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *ResizeTerminalSession {
	return &ResizeTerminalSession{sessions: sessions, resolver: resolver, devServers: devServers, agent: agent}
}

func (uc *ResizeTerminalSession) Execute(ctx context.Context, in ResizeTerminalSessionInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, in.PtyID, uc.sessions, uc.resolver, uc.devServers)
	if err != nil {
		return err
	}

	if err := uc.agent.ResizePty(ctx, devServer, in.PtyID, in.Cols, in.Rows); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_RESIZE_PTY_FAILED", "failed to resize pty", err)
	}
	if err := uc.sessions.Touch(ctx, tenantID, in.PtyID, time.Now().UTC()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TOUCH_TERMINAL_SESSION_FAILED", "failed to update terminal session activity", err)
	}
	return nil
}
