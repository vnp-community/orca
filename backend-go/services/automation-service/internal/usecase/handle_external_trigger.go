package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// HandleExternalTriggerInput mirrors HandleExternalTriggerRequest
// (proto/orca/automation/v1/automation.proto). PayloadJSON is accepted but
// currently unused — the proto comment documents it as "opaque ... passed
// through as-is, not interpreted here", and this scaffold has nothing yet
// that needs to interpret it (no per-trigger-source payload mapping is in
// scope for this task).
type HandleExternalTriggerInput struct {
	AutomationID string
	RequestID    string // the external source's own idempotency key, used verbatim
	PayloadJSON  string
}

// HandleExternalTrigger maps an external trigger source's request onto the
// same RunNow interactor every other trigger path uses (§2/§7) — it does
// not duplicate RunNow's dispatch/idempotency logic, only sets
// Trigger=RunTriggerExternal so the resulting AutomationRun records where
// the dispatch came from. See automation-service.md §7/§9: this is an
// untrusted caller boundary at the transport level, but HandleExternalTrigger
// itself trusts the caller's already-validated tenant context exactly like
// RunNow does — no separate authorization step here (no webhook
// signature/shared-secret verification is wired in this scaffold; see
// README "deviations").
type HandleExternalTrigger struct {
	runNow *RunNow
}

func NewHandleExternalTrigger(runNow *RunNow) *HandleExternalTrigger {
	return &HandleExternalTrigger{runNow: runNow}
}

func (uc *HandleExternalTrigger) Execute(ctx context.Context, in HandleExternalTriggerInput) (domain.AutomationRun, error) {
	return uc.runNow.Execute(ctx, RunNowInput{
		AutomationID: in.AutomationID,
		RequestID:    in.RequestID,
		Trigger:      domain.RunTriggerExternal,
	})
}
