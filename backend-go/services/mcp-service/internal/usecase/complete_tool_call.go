package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type CompleteInput struct {
	CallID, Result, ReasonCode string
	DurationMs                 int64
}

// CompleteToolCall finalizes the journal row of an admitted call. The same
// transaction records taint (untrusted read that succeeded) and enqueues the
// single audit event, so a crash can't lose the audit line.
type CompleteToolCall struct {
	core  *GovernanceCore
	calls ToolCallRepository
	clock Clock
}

func NewCompleteToolCall(core *GovernanceCore, calls ToolCallRepository, clock Clock) *CompleteToolCall {
	return &CompleteToolCall{core: core, calls: calls, clock: clock}
}

func (uc *CompleteToolCall) Execute(ctx context.Context, in CompleteInput) error {
	id, err := userCaller(ctx)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(in.CallID); err != nil {
		return domain.ErrNotFound()
	}
	if in.Result != domain.ResultOK && in.Result != domain.ResultError {
		return domain.ErrInvalidArgument("result must be ok or error")
	}
	if in.DurationMs < 0 {
		in.DurationMs = 0
	}
	_, err = uc.calls.FinalizeToolCall(ctx, id.TenantID, id.UserID, in.CallID, in.Result, in.ReasonCode, in.DurationMs, uc.clock.Now(), uc.core.cfg.TaintTTL)
	if err != nil {
		return wrapRepoErr(err, "failed to complete tool call")
	}
	return nil
}

// ToolCallMaintenance holds the periodic jobs of the journal.
type ToolCallMaintenance struct {
	calls     ToolCallRepository
	canceller ToolCanceller
	cfg       GovernanceConfig
	clock     Clock
	retention time.Duration
}

func NewToolCallMaintenance(calls ToolCallRepository, canceller ToolCanceller, cfg GovernanceConfig, retention time.Duration, clock Clock) *ToolCallMaintenance {
	if canceller == nil {
		canceller = noopCanceller{}
	}
	return &ToolCallMaintenance{calls: calls, canceller: canceller, cfg: cfg, clock: clock, retention: retention}
}

// ReapInterrupted finalizes calls stuck in "started" (gateway died between
// allow and complete) as interrupted, so no audit line is ever lost.
func (uc *ToolCallMaintenance) ReapInterrupted(ctx context.Context, batch int) (int, error) {
	now := uc.clock.Now()
	refs, err := uc.calls.ListStaleCalls(ctx, now.Add(-uc.cfg.ToolCallMaxAge), batch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range refs {
		ok, err := uc.calls.InterruptCall(ctx, r.TenantID, r.ID, domain.ReasonInterrupted, now)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// Purge removes finished journal rows past retention.
func (uc *ToolCallMaintenance) Purge(ctx context.Context, batch int) (int, error) {
	if uc.retention <= 0 {
		return 0, nil
	}
	return uc.calls.PurgeFinishedCalls(ctx, uc.clock.Now().Add(-uc.retention), batch)
}
