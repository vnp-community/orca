package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

// errUnknownProvider is returned by updateIssueStatus when
// LinkedIssueProvider isn't one of "jira"/"linear"/"github" — "gitlab"
// issue status sync via labels is not yet mapped (BL-PI-03's own scope
// note), and any other value is a genuinely unknown provider.
var errUnknownProvider = errors.New("issue-status-sync: unknown or unsupported issue provider")

// retryAttempts is BR-PI-08's literal "retry 3 times then give up" — exactly
// 3 total attempts, not 3 retries after the first (4 total).
const retryAttempts = 3

// syncableProviders are the trackers this service can drive correctly end to
// end. Jira only: Linear's workflow_state_id must be a state UUID (this service
// sends a name), and GitHub's label write has no per-user credential path yet.
// Other providers are skipped quietly instead of failing three times each.
var syncableProviders = map[string]bool{"jira": true}

type SyncIssueStatus struct {
	tracker         IssueTrackerClient
	scm             ScmClient
	projects        ProjectSettingsClient
	processedEvents ProcessedEventStore
	logger          *slog.Logger
}

func NewSyncIssueStatus(tracker IssueTrackerClient, scm ScmClient, projects ProjectSettingsClient, processedEvents ProcessedEventStore, logger *slog.Logger) *SyncIssueStatus {
	if logger == nil {
		logger = slog.Default()
	}
	return &SyncIssueStatus{tracker: tracker, scm: scm, projects: projects, processedEvents: processedEvents, logger: logger}
}

// HandleWorktreeLifecycle implements BR-PI-08/BR-PI-09. Runs only from the
// async consumer loop (adapter/eventbus/subscriber.go) — never from a
// synchronous RPC path, so a give-up here never propagates back to the
// worktree/PR operation that triggered the event.
func (uc *SyncIssueStatus) HandleWorktreeLifecycle(ctx context.Context, ev domain.WorktreeLifecycleEvent) error {
	if processed, err := uc.processedEvents.Seen(ctx, ev.EventID); err == nil && processed {
		return nil // JetStream at-least-once — idempotent no-op
	}
	if ev.LinkedIssueProvider == "" {
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}
	if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, ev.TenantID, ev.ProjectID); err != nil || !enabled {
		// BR-PI-07 belt-and-braces re-check: the publisher already gates
		// recording the link when sync is off, but a project's flag can
		// flip off AFTER the link was recorded and BEFORE this event is
		// processed (JetStream delivery is not instantaneous).
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}

	target := mapWorktreeEventToStatus(ev) // BL-PI-03's mapping table
	if target == (domain.TargetState{}) {
		// worktree.deleted: no mapping — see mapWorktreeEventToStatus.
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}
	if !uc.canSync(ctx, ev.EventID, ev.LinkedIssueProvider, ev.ActorUserID) {
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}

	err := doWithRetry(ctx, retryAttempts, func(ctx context.Context) error { // BR-PI-08
		return uc.updateIssueStatus(ctx, ev.TenantID, ev.ActorUserID, ev.LinkedIssueProvider, ev.LinkedIssueRef, ev.LinkedIssueSite, target)
	})
	if err != nil {
		uc.logger.ErrorContext(ctx, "gave up syncing issue status after retries", "issue", ev.LinkedIssueRef, "error", err)
		// Give up per BR-PI-08's literal words — no app-level DLQ; JetStream's
		// own redelivery window is the safety net.
	}
	return uc.processedEvents.MarkSeen(ctx, ev.EventID) // mark seen regardless of sync outcome
}

// canSync reports whether an event can be acted on at all, logging why not.
// A skipped event is deliberate and quiet, never an error: an unsupported
// provider or a missing actor will not become syncable by retrying.
func (uc *SyncIssueStatus) canSync(ctx context.Context, eventID, provider, actorUserID string) bool {
	if !syncableProviders[provider] {
		uc.logger.InfoContext(ctx, "issue status sync skipped: provider not supported", "event", eventID, "provider", provider)
		return false
	}
	if actorUserID == "" {
		// The tracker connection belongs to a user; guessing one would act on
		// someone else's account (or fail on every retry).
		uc.logger.WarnContext(ctx, "issue status sync skipped: event carries no acting user", "event", eventID)
		return false
	}
	return true
}

// HandlePullRequestLifecycle mirrors HandleWorktreeLifecycle's shape for
// pr.created -> "In Review" / pr.merged -> "Done".
func (uc *SyncIssueStatus) HandlePullRequestLifecycle(ctx context.Context, ev domain.PullRequestLifecycleEvent) error {
	if processed, err := uc.processedEvents.Seen(ctx, ev.EventID); err == nil && processed {
		return nil
	}
	if ev.LinkedIssueProvider == "" {
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}
	// NOTE: pr.created/pr.merged carry no project_id (only provider/repo/
	// pr_number) — BR-PI-07's belt-and-braces re-check is a per-PROJECT
	// flag, and PullRequestLifecycleEvent has no project reference to look
	// one up by. This is a genuine schema gap shared with
	// ScmClient.GetPullRequestForBranch's (see ports.go) — sync proceeds
	// unconditionally for PR events until a future schema revision threads
	// project_id through CreatePullRequest/MergePullRequest's outbox payload.

	target := mapPullRequestEventToStatus(ev)
	if !uc.canSync(ctx, ev.EventID, ev.LinkedIssueProvider, ev.ActorUserID) {
		return uc.processedEvents.MarkSeen(ctx, ev.EventID)
	}

	err := doWithRetry(ctx, retryAttempts, func(ctx context.Context) error {
		return uc.updateIssueStatus(ctx, ev.TenantID, ev.ActorUserID, ev.LinkedIssueProvider, ev.LinkedIssueRef, ev.LinkedIssueSite, target)
	})
	if err != nil {
		uc.logger.ErrorContext(ctx, "gave up syncing issue status after retries", "issue", ev.LinkedIssueRef, "error", err)
	}
	return uc.processedEvents.MarkSeen(ctx, ev.EventID)
}

func (uc *SyncIssueStatus) updateIssueStatus(ctx context.Context, tenantID, userID, provider, ref, site string, state domain.TargetState) error {
	switch provider {
	case "linear", "jira":
		if state.OnlyFromCategory != "" {
			category, err := uc.tracker.IssueStatusCategory(ctx, tenantID, userID, provider, ref, site)
			if err != nil {
				return err
			}
			if category != state.OnlyFromCategory {
				// Already started, finished or of unknown kind: leave it. Moving an
				// issue backwards is worse than not moving it.
				uc.logger.InfoContext(ctx, "issue status sync skipped: issue not in the expected category",
					"issue", ref, "category", category, "expected", state.OnlyFromCategory)
				return nil
			}
		}
		return uc.tracker.TransitionIssue(ctx, tenantID, userID, provider, ref, site, state.TrackerState)
	case "github":
		return uc.scm.UpdateIssue(ctx, tenantID, provider, ref, state.GitHubLabelPatch)
	default:
		return errUnknownProvider // gitlab issue status sync via labels not yet mapped
	}
}

// mapWorktreeEventToStatus implements BL-PI-03's mapping table, narrowed:
// worktree.created -> In Progress, but only for an issue still in "todo".
//
// worktree.deleted deliberately maps to NOTHING. BL-PI-03 used to map it to
// Cancelled/close when there was no open PR, but a worktree is deleted for
// routine reasons (cleanup after the PR merged, a fresh checkout, disk space),
// and the event's had_open_pr is always false. Acting on it would cancel or
// close issues whose work is finished — data loss in the tracker — so removal
// is never treated as a signal about the issue.
func mapWorktreeEventToStatus(ev domain.WorktreeLifecycleEvent) domain.TargetState {
	if ev.Deleted {
		return domain.TargetState{}
	}
	return domain.TargetState{TrackerState: "In Progress", GitHubLabelPatch: "add:in-progress", OnlyFromCategory: "todo"}
}

// mapPullRequestEventToStatus implements BL-PI-03's mapping table:
// pr.created -> In Review, pr.merged -> Done. Both apply only to an issue
// that is in progress: a todo issue was never started and a done one must
// not be reopened or re-moved (a PR may also reference an issue loosely).
func mapPullRequestEventToStatus(ev domain.PullRequestLifecycleEvent) domain.TargetState {
	if ev.Merged {
		return domain.TargetState{TrackerState: "Done", GitHubLabelPatch: "close", OnlyFromCategory: "in_progress"}
	}
	return domain.TargetState{TrackerState: "In Review", GitHubLabelPatch: "add:in-review", OnlyFromCategory: "in_progress"}
}

// doWithRetry runs fn up to attempts times with a short fixed backoff
// between attempts (BR-PI-08) — exactly attempts total tries, not
// attempts+1.
func doWithRetry(ctx context.Context, attempts int, fn func(context.Context) error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond * time.Duration(i)):
			}
		}
		if err := fn(ctx); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}
