package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// BaseRefLockAnnotator fills BaseRefLocked and BaseRefLockReason on what a read returns, with one WorktreePresence lookup for
// the whole page (no per-Request query). The lock is advisory on reads: SetRequestBaseRef decides again under the row lock.
type BaseRefLockAnnotator struct {
	Presence WorktreePresence
	Log      *slog.Logger
}

// Annotate never fails the read. When the lookup fails the Requests stay unlocked, which the write path then corrects by
// refusing with REQUEST_BASE_REF_LOCKED (or REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE) if the person tries to change the ref.
func (a *BaseRefLockAnnotator) Annotate(ctx context.Context, reqs []domain.Request) {
	if a == nil || a.Presence == nil || len(reqs) == 0 {
		return
	}
	var ids []string
	for i := range reqs {
		if reqs[i].Status.IsTerminal() {
			reqs[i].BaseRefLocked, reqs[i].BaseRefLockReason = true, domain.BaseRefLockedTerminal
			continue
		}
		ids = append(ids, reqs[i].ID)
	}
	if len(ids) == 0 {
		return
	}
	locks, err := a.Presence.HasMany(ctx, ids)
	if err != nil {
		a.log().WarnContext(ctx, "worktree presence unreadable; base ref lock left unset", slog.Int("requests", len(ids)), slog.Any("error", err))
		return
	}
	for i := range reqs {
		if reason, ok := locks[reqs[i].ID]; ok && reason != domain.BaseRefUnlocked {
			reqs[i].BaseRefLocked, reqs[i].BaseRefLockReason = true, reason
		}
	}
}

func (a *BaseRefLockAnnotator) log() *slog.Logger {
	if a.Log != nil {
		return a.Log
	}
	return slog.Default()
}
