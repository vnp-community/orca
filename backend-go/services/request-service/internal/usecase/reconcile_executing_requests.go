package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	reconcileBatch = 50
	// reconcileLease only has to outlast one evaluation; a crashed replica's lease then expires on its own.
	reconcileLease = 2 * time.Minute
)

// ReconcileExecutingRequests is the safety net under the event-driven loop: it re-evaluates executing Requests
// that have been quiet, which repairs lost events (bulk release of unlinked tasks emits none, NATS can be down at
// start, a consumer can be off).
type ReconcileExecutingRequests struct {
	Scanner  ExecutingRequestScanner
	Leases   ReconcileLeases
	Requests RequestReader
	Evaluate *EvaluateExecution
	Settings ExecutionSettings
	// Owner names this replica in the lease table.
	Owner string
	Log   *slog.Logger
}

func (uc *ReconcileExecutingRequests) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

// Execute evaluates every quiet executing Request it can lease and returns how many it handled.
func (uc *ReconcileExecutingRequests) Execute(ctx context.Context) (int, error) {
	quiet := uc.Settings.normalized().ReconcileQuiet
	owner := uc.Owner
	if owner == "" {
		owner = uuid.NewString()
	}
	refs, err := uc.Scanner.ListQuietExecuting(ctx, quiet, reconcileBatch)
	if err != nil {
		return 0, err
	}
	handled := 0
	for _, ref := range refs {
		if ctx.Err() != nil {
			return handled, ctx.Err()
		}
		ok, err := uc.reconcileOne(tenant.WithTenantID(ctx, ref.TenantID), ref, owner, quiet)
		if err != nil {
			uc.log().Warn("reconcile of a request failed", slog.String("request_id", ref.RequestID), slog.String("error", truncateForLog(err.Error())))
		}
		if ok {
			handled++
		}
	}
	return handled, nil
}

func (uc *ReconcileExecutingRequests) reconcileOne(ctx context.Context, ref ExecutingRef, owner string, quiet time.Duration) (bool, error) {
	claimed, err := uc.Leases.Claim(ctx, ref.RequestID, owner, reconcileLease, quiet)
	if err != nil || !claimed {
		return false, err
	}
	defer func() {
		if err := uc.Leases.Release(ctx, ref.RequestID, owner); err != nil {
			uc.log().Warn("releasing a reconcile lease failed", slog.String("request_id", ref.RequestID), slog.Any("error", err))
		}
	}()
	req, err := uc.Requests.Get(ctx, ref.RequestID)
	if err != nil {
		return true, err
	}
	if req.Status != domain.RequestStatusExecuting {
		return true, nil
	}
	return true, uc.Evaluate.Run(ctx, req)
}

// RunReconcileLoop ticks until ctx ends. The first pass waits one interval so a restart does not stampede.
func (uc *ReconcileExecutingRequests) RunReconcileLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if n, err := uc.Execute(ctx); err != nil {
			uc.log().Warn("reconcile pass failed", slog.Any("error", err))
		} else if n > 0 {
			uc.log().Info("reconciled executing requests", slog.Int("count", n))
		}
	}
}
