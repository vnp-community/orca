package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

// StatusNames are the tracker transition names a Request drives. They are
// configuration because Jira workflows are customizable.
type StatusNames struct {
	InProgress string
	Done       string
}

func DefaultStatusNames() StatusNames { return StatusNames{InProgress: "In Progress", Done: "Done"} }

// Option customizes SyncIssueStatus for Request-driven sync.
type Option func(*SyncIssueStatus)

func WithStatusNames(n StatusNames) Option {
	return func(uc *SyncIssueStatus) {
		def := DefaultStatusNames()
		if n.InProgress == "" {
			n.InProgress = def.InProgress
		}
		if n.Done == "" {
			n.Done = def.Done
		}
		uc.names = n
	}
}

func WithRequestSyncState(s RequestSyncStateStore) Option {
	return func(uc *SyncIssueStatus) { uc.syncState = s }
}

// WithRequestLookup enables the single-writer check; nil keeps worktree/PR sync unconditional.
func WithRequestLookup(c RequestLookupClient) Option {
	return func(uc *SyncIssueStatus) { uc.requests = c }
}

// WithObserver sets the metrics sink; nil keeps the no-op default.
func WithObserver(o SyncObserver) Option {
	return func(uc *SyncIssueStatus) {
		if o != nil {
			uc.observer = o
		}
	}
}

// WithIssueComments turns on the optional Jira comment; orcaBaseURL (may be empty) is used for the link.
func WithIssueComments(c IssueCommenter, orcaBaseURL string) Option {
	return func(uc *SyncIssueStatus) { uc.commenter, uc.orcaBaseURL = c, strings.TrimRight(orcaBaseURL, "/") }
}

const eventStatusChanged, eventCompleted = "status_changed", "completed"

const requestStatusBacklog = "request_backlog"

// HandleRequestStatus syncs a Request status change to its Jira issue. A
// returned error means "redeliver" (bounded by maxTransientDeliveries); every
// other outcome is acknowledged via MarkSeen.
func (uc *SyncIssueStatus) HandleRequestStatus(ctx context.Context, ev domain.RequestStatusEvent) error {
	if processed, err := uc.processedEvents.Seen(ctx, ev.EventID); err == nil && processed {
		return nil
	}
	kind := eventStatusChanged
	if ev.Completed {
		kind = eventCompleted
	}
	done := func(result string) error {
		if result != "" {
			uc.observer.ObserveRequestEvent(kind, result)
		}
		uc.deliveries.forget(ev.EventID)
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}

	if ev.SourceProvider != "jira" {
		return done("skipped_not_jira")
	}
	// Flag handling: the request side does not emit when the flow is off (SOL-024 Q2), so no flag check here.
	target := mapRequestEventToStatus(ev, uc.names)
	userID := requestActor(ev)
	if target.TrackerState == "" {
		// Backlog only gets the optional comment; every other unmapped change is ignored silently.
		if ev.To == requestStatusBacklog && !ev.Completed {
			uc.commentOnRequest(ctx, ev, userID)
		}
		return done("")
	}
	if userID == "" {
		uc.logger.WarnContext(ctx, "request status sync skipped: no user to act as", "event", ev.EventID)
		return done("skipped_no_actor")
	}

	// Advance before the Jira call: prefer a missed transition (healed by the later
	// completed event) over a repeated one. Version 0 means the publisher sent none.
	if uc.syncState != nil && ev.Version > 0 {
		applied, err := uc.syncState.Advance(ctx, ev.TenantID, ev.RequestID, ev.Version, target.TrackerState)
		if err != nil {
			if uc.deliveries.next(ev.EventID) < maxTransientDeliveries {
				return err
			}
			uc.logger.ErrorContext(ctx, "gave up recording request sync state", "event", ev.EventID, "error", err)
			return done("failed")
		}
		if !applied {
			uc.logger.InfoContext(ctx, "request status sync skipped: stale version", "event", ev.EventID, "version", ev.Version)
			return done("skipped_stale")
		}
	}

	var transitioned bool
	start := time.Now()
	err := doWithRetry(ctx, retryAttempts, func(ctx context.Context) error {
		var err error
		transitioned, err = uc.applyIssueStatus(ctx, ev.TenantID, userID, ev.SourceProvider, ev.SourceRef, ev.SourceSite, target)
		return err
	})
	switch {
	case err == nil && !transitioned:
		return done("skipped_category")
	case err == nil:
		uc.observer.ObserveJiraTransition(time.Since(start))
		uc.commentOnRequest(ctx, ev, userID)
		return done("applied")
	case isTransitionUnavailable(err):
		uc.logger.WarnContext(ctx, "request status sync skipped: workflow has no such transition", "issue", ev.SourceRef, "target", target.TrackerState)
		return done("skipped_transition_unavailable")
	default:
		uc.logger.ErrorContext(ctx, "gave up syncing request status after retries", "issue", ev.SourceRef, "error", err)
		return done("failed")
	}
}

// isTransitionUnavailable matches the Jira adapter's ErrTransitionUnavailable
// text: the sentinel does not survive the gRPC hop, only its message does.
func isTransitionUnavailable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "transition unavailable")
}

// requestActor picks whose Jira credential to use: the acting user, else the reporter (AI/system actors have none).
func requestActor(ev domain.RequestStatusEvent) string {
	if ev.ActorKind == "user" && ev.ActorID != "" {
		return ev.ActorID
	}
	return ev.ReporterID
}

// mapRequestEventToStatus is the Request to Jira table (CR-REQ-024 2.2). An
// empty TargetState means "do nothing": backlog, cancelled, awaiting_*,
// classifying, planning and type changes never move the issue.
func mapRequestEventToStatus(ev domain.RequestStatusEvent, names StatusNames) domain.TargetState {
	if ev.Completed {
		return domain.TargetState{TrackerState: names.Done, OnlyFromCategories: []string{"todo", "in_progress"}}
	}
	analysisOnly := ev.Type == "spike" || ev.Type == "question" // these complete after analyzing, never execute
	switch {
	case ev.To == "executing" && !analysisOnly, ev.To == "analyzing" && analysisOnly:
		return domain.TargetState{TrackerState: names.InProgress, OnlyFromCategories: []string{"todo"}}
	}
	return domain.TargetState{}
}

// commentOnRequest is best effort and carries only number, type, status and a link, never title/body/solution text.
func (uc *SyncIssueStatus) commentOnRequest(ctx context.Context, ev domain.RequestStatusEvent, userID string) {
	if uc.commenter == nil || userID == "" {
		return
	}
	body := fmt.Sprintf("Orca request #%d (%s) is now %s.", ev.Number, ev.Type, ev.To)
	if ev.Completed {
		body = fmt.Sprintf("Orca request #%d (%s) is completed.", ev.Number, ev.Type)
	}
	if uc.orcaBaseURL != "" {
		body += fmt.Sprintf(" %s/requests/%s", uc.orcaBaseURL, ev.RequestID)
	}
	if err := uc.commenter.AddComment(ctx, ev.TenantID, userID, ev.SourceProvider, ev.SourceRef, ev.SourceSite, body); err != nil {
		uc.logger.WarnContext(ctx, "request jira comment failed", "issue", ev.SourceRef, "error", err)
		uc.observer.ObserveRequestEvent("comment", "failed")
	}
}
