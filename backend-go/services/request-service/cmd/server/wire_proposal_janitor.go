package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	proposalJanitorStartupDelay = time.Minute
	proposalJanitorInterval     = 10 * time.Minute
)

// startProposalJanitor sweeps the proposal worktrees until ctx ends: those of cancelled and finished Requests, and a failed
// run's leftover once its retention has passed. A pass that fails is logged and tried again at the next tick.
func startProposalJanitor(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, j *usecase.ProposalJanitor) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		wait := proposalJanitorStartupDelay
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			sum, err := j.Sweep(ctx)
			switch {
			case err != nil && ctx.Err() == nil:
				log.Warn("proposal cleanup pass failed", slog.Any("error", err))
			case sum.Removed > 0 || sum.Failed > 0:
				log.Info("proposal cleanup pass", slog.Int("tenants", sum.Tenants), slog.Int("looked", sum.Looked), slog.Int("removed", sum.Removed),
					slog.Int("abandoned", sum.Abandoned), slog.Int("failed", sum.Failed))
			}
			wait = proposalJanitorInterval
		}
	}()
}
