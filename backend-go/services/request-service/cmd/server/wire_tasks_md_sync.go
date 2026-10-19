package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	tasksMdSyncStartupDelay = 45 * time.Second
	tasksMdSyncInterval     = 30 * time.Second
)

// taskSyncSlot lets the execution loop, which is built first, notify a syncer that wirePlan builds later. Until it is set a
// succeeded task is only recorded; the sweep catches up from task state.
type taskSyncSlot struct {
	p atomic.Pointer[usecase.TasksMdSyncer]
}

func (s *taskSyncSlot) set(t *usecase.TasksMdSyncer) { s.p.Store(t) }

func (s *taskSyncSlot) OnTaskSucceeded(ctx context.Context, requestID string) error {
	if t := s.p.Load(); t != nil {
		return t.OnTaskSucceeded(ctx, requestID)
	}
	return nil
}

// startTasksMdSync ticks tasks.md of OpenSpec changes and archives those of completed Requests until ctx ends.
func startTasksMdSync(ctx context.Context, wg *sync.WaitGroup, s *usecase.TasksMdSyncer) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			return
		case <-time.After(tasksMdSyncStartupDelay):
		}
		s.Run(ctx, tasksMdSyncInterval)
	}()
}
