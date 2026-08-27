// Package eventbus implements issue-status-sync's consumer side —
// subscribing to project-service's worktree.created/worktree.deleted and
// scm-integration-service's pull_request.created/pull_request.merged
// streams and dispatching to usecase.SyncIssueStatus. This service only
// consumes; it has no outbox relay of its own (SOL-PI-03).
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

// Subject/stream names — must match project-service's/scm-integration-service's
// own EnsureStream calls exactly.
const (
	worktreeCreatedSubject = "orca.project.worktree.created"
	worktreeDeletedSubject = "orca.project.worktree.deleted"
	prCreatedSubject       = "orca.scm.pull_request.created"
	prMergedSubject        = "orca.scm.pull_request.merged"

	projectStream = "PROJECT"
	scmStream     = "SCM"

	// consumerName is a stable durable-consumer name shared by every
	// replica of this service — JetStream load-balances each event to
	// exactly one replica (see common/eventbus.Consumer.Subscribe's doc
	// comment), the correct at-least-once/effectively-once shape for a
	// side-effecting consumer like this one.
	consumerName = "issue-status-sync"
)

// worktreeLifecycleWirePayload mirrors project-service's
// worktreeLifecycleEventPayload JSON shape exactly.
type worktreeLifecycleWirePayload struct {
	WorktreeID          string `json:"worktree_id"`
	ProjectID           string `json:"project_id"`
	LinkedIssueProvider string `json:"linked_issue_provider"`
	LinkedIssueRef      string `json:"linked_issue_ref"`
	HadOpenPr           bool   `json:"had_open_pr"`
}

// prLifecycleWirePayload mirrors scm-integration-service's
// prLifecycleEventPayload JSON shape exactly.
type prLifecycleWirePayload struct {
	Provider            string `json:"provider"`
	Repo                string `json:"repo"`
	PrNumber            int32  `json:"pr_number"`
	LinkedIssueProvider string `json:"linked_issue_provider"`
	LinkedIssueRef      string `json:"linked_issue_ref"`
}

// Subscriber wires common/eventbus.Consumer to usecase.SyncIssueStatus.
type Subscriber struct {
	consumer *eventbus.Consumer
	sync     *usecase.SyncIssueStatus
	logger   *slog.Logger
}

func New(consumer *eventbus.Consumer, sync *usecase.SyncIssueStatus, logger *slog.Logger) *Subscriber {
	if logger == nil {
		logger = slog.Default()
	}
	return &Subscriber{consumer: consumer, sync: sync, logger: logger}
}

// Run starts all four subscriptions and blocks until ctx is cancelled or
// one of them returns a fatal (non-context) error. Each subscription runs
// in its own goroutine — a stall on one subject must not block the others.
func (s *Subscriber) Run(ctx context.Context) error {
	errCh := make(chan error, 4)
	subs := []struct {
		stream, subject string
		handle          eventbus.Handler
	}{
		{projectStream, worktreeCreatedSubject, s.handleWorktreeEvent(false)},
		{projectStream, worktreeDeletedSubject, s.handleWorktreeEvent(true)},
		{scmStream, prCreatedSubject, s.handlePullRequestEvent(false)},
		{scmStream, prMergedSubject, s.handlePullRequestEvent(true)},
	}
	for _, sub := range subs {
		stream, subject, handle := sub.stream, sub.subject, sub.handle
		go func() {
			if err := s.consumer.Subscribe(ctx, stream, consumerName+"-"+subject, subject, handle); err != nil {
				errCh <- fmt.Errorf("subscribing to %s: %w", subject, err)
			}
		}()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Subscriber) handleWorktreeEvent(deleted bool) eventbus.Handler {
	return func(ctx context.Context, event eventbus.Event) error {
		var wire worktreeLifecycleWirePayload
		if err := json.Unmarshal(event.Payload, &wire); err != nil {
			s.logger.ErrorContext(ctx, "malformed worktree lifecycle event payload", "error", err)
			return nil // NAK-then-drop would just redeliver the same malformed payload forever
		}
		ev := domain.WorktreeLifecycleEvent{
			EventID: event.ID, TenantID: event.TenantID, SchemaVersion: int32(event.Version),
			OccurredAt: event.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
			WorktreeID: wire.WorktreeID, ProjectID: wire.ProjectID,
			LinkedIssueProvider: wire.LinkedIssueProvider, LinkedIssueRef: wire.LinkedIssueRef,
			HadOpenPR: wire.HadOpenPr, Deleted: deleted,
		}
		return s.sync.HandleWorktreeLifecycle(ctx, ev)
	}
}

func (s *Subscriber) handlePullRequestEvent(merged bool) eventbus.Handler {
	return func(ctx context.Context, event eventbus.Event) error {
		var wire prLifecycleWirePayload
		if err := json.Unmarshal(event.Payload, &wire); err != nil {
			s.logger.ErrorContext(ctx, "malformed pull request lifecycle event payload", "error", err)
			return nil
		}
		ev := domain.PullRequestLifecycleEvent{
			EventID: event.ID, TenantID: event.TenantID, SchemaVersion: int32(event.Version),
			OccurredAt: event.OccurredAt.Format("2006-01-02T15:04:05Z07:00"),
			Provider:   wire.Provider, Repo: wire.Repo, PRNumber: wire.PrNumber,
			LinkedIssueProvider: wire.LinkedIssueProvider, LinkedIssueRef: wire.LinkedIssueRef,
			Merged: merged,
		}
		return s.sync.HandlePullRequestLifecycle(ctx, ev)
	}
}
