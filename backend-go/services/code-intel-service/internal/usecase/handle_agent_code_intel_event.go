package usecase

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/types/known/timestamppb"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// AgentEventPayload captures normalized event fields received from dev server agents.
type AgentEventPayload struct {
	Kind          string // index_changed | reindex_progress | quality_progress | quality_finished | resync | overflow
	WorkspaceRoot string
	Tool          string
	Commit        string
	IndexedAt     string
	JobID         string
	Stage         string
	Percent       *int32
	Message       string
	State         string // queued | running | succeeded | failed | cancelled
	Reason        string
	PayloadJSON   string
}

// PushBroadcaster defines the interface for publishing CodeIntelPush events to active streaming clients.
type PushBroadcaster interface {
	Publish(push *codeintelv1.CodeIntelPush)
}

// EventOutboxWriter publishes events via the transactional outbox table.
type EventOutboxWriter interface {
	WriteOutbox(ctx context.Context, tenantID, subject, eventID string, payload any) error
}

// ProcessedEventsStore tracks processed event IDs for deduplication.
type ProcessedEventsStore interface {
	MarkProcessed(ctx context.Context, tenantID, eventID string) (bool, error)
}

// AgentCodeIntelEventHandler routes and handles incoming agent events (TASK-024-05 & 06).
type AgentCodeIntelEventHandler struct {
	targets         BindingStreamTargets
	invalidator     *SnapshotInvalidator
	coalescer       *IndexEventCoalescer
	broadcaster     PushBroadcaster
	outbox          EventOutboxWriter
	processedStore  ProcessedEventsStore
	reindexStore    ReindexJobStore
	qualitySink     QualityRunEventSink
	rateLimiter     *DevServerRateLimiter
	logger          *slog.Logger
}

// NewAgentCodeIntelEventHandler creates a new event handler.
func NewAgentCodeIntelEventHandler(
	targets BindingStreamTargets,
	invalidator *SnapshotInvalidator,
	coalescer *IndexEventCoalescer,
	broadcaster PushBroadcaster,
	outbox EventOutboxWriter,
	processedStore ProcessedEventsStore,
	reindexStore ReindexJobStore,
	qualitySink QualityRunEventSink,
	rateLimiter *DevServerRateLimiter,
	logger *slog.Logger,
) *AgentCodeIntelEventHandler {
	if qualitySink == nil {
		qualitySink = &NoopQualityRunEventSink{}
	}
	if rateLimiter == nil {
		rateLimiter = NewDevServerRateLimiter(20.0, 20)
	}

	h := &AgentCodeIntelEventHandler{
		targets:        targets,
		invalidator:    invalidator,
		coalescer:      coalescer,
		broadcaster:    broadcaster,
		outbox:         outbox,
		processedStore: processedStore,
		reindexStore:   reindexStore,
		qualitySink:    qualitySink,
		rateLimiter:    rateLimiter,
		logger:         logger,
	}

	return h
}

// HandleEvent processes an incoming event from a dev server.
func (h *AgentCodeIntelEventHandler) HandleEvent(ctx context.Context, tenantID, devServerID string, p AgentEventPayload) error {
	// Check rate limit per dev server
	if !h.rateLimiter.Allow(devServerID) {
		// Rate limit exceeded: trigger resync
		p.Kind = "resync"
	}

	var target *BindingStreamTarget
	if h.targets != nil {
		var err error
		target, err = h.targets.FindTarget(ctx, tenantID, devServerID, p.WorkspaceRoot)
		if err != nil {
			return err
		}
	}
	if target == nil {
		target = &BindingStreamTarget{
			TenantID:      tenantID,
			DevServerID:   devServerID,
			RepoBindingID: p.WorkspaceRoot,
			WorkspaceRoot: p.WorkspaceRoot,
		}
	}

	switch p.Kind {
	case "index_changed":
		return h.handleIndexChanged(ctx, target, p)

	case "reindex_progress":
		return h.handleReindexProgress(ctx, target, p)

	case "quality_progress":
		return h.handleQualityProgress(ctx, target, p)

	case "quality_finished":
		return h.handleQualityFinished(ctx, target, p)

	case "resync", "overflow":
		return h.handleResyncOrOverflow(ctx, target, p)

	default:
		return nil
	}
}

func (h *AgentCodeIntelEventHandler) handleIndexChanged(ctx context.Context, target *BindingStreamTarget, p AgentEventPayload) error {
	if h.invalidator != nil {
		h.invalidator.InvalidateProbe(target.TenantID, target.RepoBindingID)
	}

	// Ingest into coalescer if available, otherwise process directly
	if h.coalescer != nil {
		h.coalescer.Ingest(ctx, target.TenantID, target.RepoBindingID, p.Tool, p.Commit, p.IndexedAt, p.Kind)
		return nil
	}

	return h.FlushCoalescedEvent(ctx, CoalescedEvent{
		TenantID:      target.TenantID,
		BindingID:     target.RepoBindingID,
		Tools:         map[string]bool{p.Tool: true},
		NewestCommit:  p.Commit,
		NewestIndexed: p.IndexedAt,
		Kind:          p.Kind,
	}, target)
}

// FlushCoalescedEvent executes invalidation, outbox write, and push for a debounced index event.
func (h *AgentCodeIntelEventHandler) FlushCoalescedEvent(ctx context.Context, e CoalescedEvent, target *BindingStreamTarget) error {
	tools := make([]string, 0, len(e.Tools))
	for tool := range e.Tools {
		tools = append(tools, tool)
	}

	eventID := DeterministicEventID(e.TenantID, e.BindingID, tools, e.NewestCommit, e.NewestIndexed, "changed")

	if h.processedStore != nil {
		processed, err := h.processedStore.MarkProcessed(ctx, e.TenantID, eventID)
		if err == nil && !processed {
			// Duplicate event already processed
			return nil
		}
	}

	if h.invalidator != nil {
		_ = h.invalidator.InvalidateBinding(ctx, e.TenantID, e.BindingID, "index_changed")
	}

	if h.outbox != nil {
		_ = h.outbox.WriteOutbox(ctx, e.TenantID, "orca.codeintel.index.changed", eventID, map[string]any{
			"binding":   e.BindingID,
			"commit":    e.NewestCommit,
			"indexedAt": e.NewestIndexed,
			"tools":     tools,
		})
	}

	if h.broadcaster != nil {
		wtID := ""
		projID := ""
		if target != nil {
			wtID = target.WorktreeID
			projID = target.ProjectID
		}
		h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
			Kind:          "changed",
			EventId:       eventID,
			WorktreeId:    wtID,
			RepoBindingId: e.BindingID,
			ProjectId:     projID,
			Reason:        "index_changed",
			Tools:         tools,
			Commit:        e.NewestCommit,
			IndexedAt:     e.NewestIndexed,
			OccurredAt:    timestamppb.Now(),
		})
	}

	return nil
}

func (h *AgentCodeIntelEventHandler) handleReindexProgress(ctx context.Context, target *BindingStreamTarget, p AgentEventPayload) error {
	if h.broadcaster != nil {
		h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
			Kind:          "reindex_progress",
			WorktreeId:    target.WorktreeID,
			RepoBindingId: target.RepoBindingID,
			ProjectId:     target.ProjectID,
			JobId:         p.JobID,
			Stage:         p.Stage,
			Percent:       p.Percent,
			Message:       p.Message,
			State:         p.State,
			OccurredAt:    timestamppb.Now(),
		})
	}

	// On terminal state, invalidate cache and emit final changed push
	if p.State == "succeeded" || p.State == "failed" || p.State == "cancelled" {
		if h.invalidator != nil {
			_ = h.invalidator.InvalidateBinding(ctx, target.TenantID, target.RepoBindingID, "reindex_finished")
		}

		eventID := DeterministicEventID(target.TenantID, target.RepoBindingID, []string{p.Tool}, p.Commit, p.IndexedAt, "reindex_finished")
		if h.outbox != nil {
			_ = h.outbox.WriteOutbox(ctx, target.TenantID, "orca.codeintel.reindex.finished", eventID, map[string]any{
				"binding": target.RepoBindingID,
				"jobId":   p.JobID,
				"state":   p.State,
			})
		}

		if h.broadcaster != nil {
			h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
				Kind:          "changed",
				EventId:       eventID,
				WorktreeId:    target.WorktreeID,
				RepoBindingId: target.RepoBindingID,
				ProjectId:     target.ProjectID,
				Reason:        "reindex_finished",
				JobId:         p.JobID,
				State:         p.State,
				OccurredAt:    timestamppb.Now(),
			})
		}
	}

	return nil
}

func (h *AgentCodeIntelEventHandler) handleQualityProgress(ctx context.Context, target *BindingStreamTarget, p AgentEventPayload) error {
	if h.broadcaster != nil {
		h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
			Kind:          "quality_progress",
			WorktreeId:    target.WorktreeID,
			RepoBindingId: target.RepoBindingID,
			ProjectId:     target.ProjectID,
			JobId:         p.JobID,
			Stage:         p.Stage,
			Percent:       p.Percent,
			Message:       p.Message,
			OccurredAt:    timestamppb.Now(),
		})
	}
	return nil
}

func (h *AgentCodeIntelEventHandler) handleQualityFinished(ctx context.Context, target *BindingStreamTarget, p AgentEventPayload) error {
	if h.broadcaster != nil {
		h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
			Kind:          "quality_finished",
			WorktreeId:    target.WorktreeID,
			RepoBindingId: target.RepoBindingID,
			ProjectId:     target.ProjectID,
			JobId:         p.JobID,
			PayloadJson:   p.PayloadJSON,
			OccurredAt:    timestamppb.Now(),
		})
	}

	if h.qualitySink != nil {
		_ = h.qualitySink.OnQualityFinished(ctx, target.TenantID, target.DevServerID, target.WorkspaceRoot, p.JobID, p.PayloadJSON)
	}

	return nil
}

func (h *AgentCodeIntelEventHandler) handleResyncOrOverflow(ctx context.Context, target *BindingStreamTarget, p AgentEventPayload) error {
	if h.invalidator != nil {
		h.invalidator.InvalidateProbe(target.TenantID, target.RepoBindingID)
	}

	if h.broadcaster != nil {
		h.broadcaster.Publish(&codeintelv1.CodeIntelPush{
			Kind:          "changed",
			WorktreeId:    target.WorktreeID,
			RepoBindingId: target.RepoBindingID,
			ProjectId:     target.ProjectID,
			Reason:        p.Kind,
			Resync:        true,
			OccurredAt:    timestamppb.Now(),
		})
	}
	return nil
}
