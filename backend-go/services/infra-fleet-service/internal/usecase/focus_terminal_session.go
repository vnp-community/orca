package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// FocusTerminalSession is a bookkeeping touch only — per
// FocusTerminalSessionRequest's proto doc comment, it does not call the
// agent at all (unlike Resize/Kill/Stop), it just records that the caller
// looked at/switched to this pane.
type FocusTerminalSession struct {
	sessions TerminalSessionRepository
}

func NewFocusTerminalSession(sessions TerminalSessionRepository) *FocusTerminalSession {
	return &FocusTerminalSession{sessions: sessions}
}

func (uc *FocusTerminalSession) Execute(ctx context.Context, ptyID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, _, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TERMINAL_LOOKUP_FAILED", "failed to look up terminal session", err)
	}
	if !found {
		return apperrors.New(apperrors.KindNotFound, "INFRA_TERMINAL_NOT_FOUND", "terminal session not found", nil)
	}

	if err := uc.sessions.Touch(ctx, tenantID, ptyID, time.Now().UTC()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TOUCH_TERMINAL_SESSION_FAILED", "failed to update terminal session activity", err)
	}
	return nil
}
