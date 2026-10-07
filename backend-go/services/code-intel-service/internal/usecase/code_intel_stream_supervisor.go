package usecase

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// DevServerStreamListener receives events from an active stream connection.
type DevServerStreamListener interface {
	ListenStream(ctx context.Context, tenantID, devServerID string, onEvent func(AgentEventPayload)) error
}

// CodeIntelStreamSupervisor manages persistent streams from dev servers (TASK-024-03).
type CodeIntelStreamSupervisor struct {
	targets           BindingStreamTargets
	listener          DevServerStreamListener
	handler           *AgentCodeIntelEventHandler
	reconcileInterval time.Duration
	logger            *slog.Logger

	mu      sync.Mutex
	active  map[string]context.CancelFunc // key: tenant + ":" + devServerID
	wg      sync.WaitGroup
}

// NewCodeIntelStreamSupervisor creates a new supervisor.
func NewCodeIntelStreamSupervisor(
	targets BindingStreamTargets,
	listener DevServerStreamListener,
	handler *AgentCodeIntelEventHandler,
	reconcileInterval time.Duration,
	logger *slog.Logger,
) *CodeIntelStreamSupervisor {
	if reconcileInterval <= 0 {
		reconcileInterval = 60 * time.Second
	}
	return &CodeIntelStreamSupervisor{
		targets:           targets,
		listener:          listener,
		handler:           handler,
		reconcileInterval: reconcileInterval,
		logger:            logger,
		active:            make(map[string]context.CancelFunc),
	}
}

// Start runs the periodic reconciliation loop until ctx is canceled.
func (s *CodeIntelStreamSupervisor) Start(ctx context.Context) {
	s.Reconcile(ctx)

	ticker := time.NewTicker(s.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.StopAll()
			return
		case <-ticker.C:
			s.Reconcile(ctx)
		}
	}
}

// Reconcile checks for new dev server targets and spins up stream workers.
func (s *CodeIntelStreamSupervisor) Reconcile(ctx context.Context) {
	if s.targets == nil || s.listener == nil {
		return
	}

	targets, err := s.targets.ListTargets(ctx)
	if err != nil {
		return
	}

	desired := make(map[string]BindingStreamTarget)
	for _, t := range targets {
		key := t.TenantID + ":" + t.DevServerID
		desired[key] = t
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop obsolete streams
	for key, cancel := range s.active {
		if _, exists := desired[key]; !exists {
			cancel()
			delete(s.active, key)
		}
	}

	// Start new streams
	for key, t := range desired {
		if _, running := s.active[key]; !running {
			streamCtx, cancel := context.WithCancel(ctx)
			s.active[key] = cancel
			s.wg.Add(1)
			go s.runStreamWorker(streamCtx, t.TenantID, t.DevServerID)
		}
	}
}

func (s *CodeIntelStreamSupervisor) runStreamWorker(ctx context.Context, tenantID, devServerID string) {
	defer s.wg.Done()

	backoffSteps := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second}
	step := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := s.listener.ListenStream(ctx, tenantID, devServerID, func(p AgentEventPayload) {
			step = 0 // Reset backoff on receiving event
			if s.handler != nil {
				_ = s.handler.HandleEvent(ctx, tenantID, devServerID, p)
			}
		})

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			wait := backoffSteps[step]
			if step < len(backoffSteps)-1 {
				step++
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
	}
}

// StopAll terminates all background stream goroutines and waits for completion.
func (s *CodeIntelStreamSupervisor) StopAll() {
	s.mu.Lock()
	for _, cancel := range s.active {
		cancel()
	}
	s.active = make(map[string]context.CancelFunc)
	s.mu.Unlock()

	s.wg.Wait()
}
