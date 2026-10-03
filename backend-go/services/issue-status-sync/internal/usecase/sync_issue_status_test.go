package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

// In-memory fakes — the "test against fakes, not a real gRPC client" pattern
// this codebase's usecase tests already establish.

type fakeTracker struct {
	calls       int
	failN       int // fail the first failN transition calls, then succeed
	category    string
	categoryErr error
	gotUser     string
	gotState    string
	gotRef      string
	catCalls    int
}

func (f *fakeTracker) TransitionIssue(_ context.Context, _, userID, _, ref, state string) error {
	f.calls++
	f.gotUser, f.gotState, f.gotRef = userID, state, ref
	if f.calls <= f.failN {
		return errors.New("transient failure")
	}
	return nil
}

func (f *fakeTracker) IssueStatusCategory(_ context.Context, _, _, _, _ string) (string, error) {
	f.catCalls++
	return f.category, f.categoryErr
}

type fakeScm struct {
	calls int
}

func (f *fakeScm) UpdateIssue(context.Context, string, string, string, string) error {
	f.calls++
	return nil
}

func (f *fakeScm) GetPullRequestForBranch(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

type fakeProjects struct {
	enabled bool
	err     error
	calls   int
}

func (f *fakeProjects) IsIssueStatusSyncEnabled(context.Context, string, string) (bool, error) {
	f.calls++
	return f.enabled, f.err
}

type fakeProcessedEvents struct {
	seen   map[string]bool
	marked []string
}

func newFakeProcessedEvents() *fakeProcessedEvents {
	return &fakeProcessedEvents{seen: map[string]bool{}}
}

func (f *fakeProcessedEvents) Seen(_ context.Context, eventID string) (bool, error) {
	return f.seen[eventID], nil
}

func (f *fakeProcessedEvents) MarkSeen(_ context.Context, eventID string) error {
	f.seen[eventID] = true
	f.marked = append(f.marked, eventID)
	return nil
}

type syncHarness struct {
	uc        *SyncIssueStatus
	tracker   *fakeTracker
	scm       *fakeScm
	projects  *fakeProjects
	processed *fakeProcessedEvents
}

func newHarness() *syncHarness {
	h := &syncHarness{tracker: &fakeTracker{category: "todo"}, scm: &fakeScm{}, projects: &fakeProjects{enabled: true}, processed: newFakeProcessedEvents()}
	h.uc = NewSyncIssueStatus(h.tracker, h.scm, h.projects, h.processed, nil)
	return h
}

func (h *syncHarness) providerCalls() int { return h.tracker.calls + h.scm.calls }

func jiraCreated(id string) domain.WorktreeLifecycleEvent {
	return domain.WorktreeLifecycleEvent{
		EventID: id, TenantID: "t1", ProjectID: "p1",
		LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1", ActorUserID: "user-7",
	}
}

func TestHandleWorktreeLifecycle_DuplicateEventIsNoOp(t *testing.T) {
	h := newHarness()
	h.processed.seen["ev-1"] = true

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatal(err)
	}
	if h.providerCalls() != 0 || h.tracker.catCalls != 0 {
		t.Errorf("a duplicate event must touch nothing, got calls=%d cat=%d", h.providerCalls(), h.tracker.catCalls)
	}
}

func TestHandleWorktreeLifecycle_EmptyLinkedIssueMarksSeenWithoutCalling(t *testing.T) {
	h := newHarness()
	ev := jiraCreated("ev-1")
	ev.LinkedIssueProvider = ""

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if h.providerCalls() != 0 || !h.processed.seen["ev-1"] {
		t.Errorf("want no calls and event marked seen; calls=%d seen=%v", h.providerCalls(), h.processed.seen["ev-1"])
	}
}

func TestHandleWorktreeLifecycle_SyncDisabledMarksSeenWithoutCalling(t *testing.T) {
	h := newHarness()
	h.projects.enabled = false

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatal(err)
	}
	if h.providerCalls() != 0 || h.tracker.catCalls != 0 || !h.processed.seen["ev-1"] {
		t.Errorf("a project with sync off must be left alone; calls=%d cat=%d", h.providerCalls(), h.tracker.catCalls)
	}
}

func TestHandleWorktreeLifecycle_CreatedMovesTodoIssueToInProgressAsTheActor(t *testing.T) {
	h := newHarness()

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.calls != 1 || h.tracker.gotState != "In Progress" || h.tracker.gotRef != "ENG-1" {
		t.Fatalf("want one In Progress transition for ENG-1, got calls=%d state=%q ref=%q", h.tracker.calls, h.tracker.gotState, h.tracker.gotRef)
	}
	if h.tracker.gotUser != "user-7" {
		t.Errorf("the call must run as the actor, got %q", h.tracker.gotUser)
	}
	if !h.processed.seen["ev-1"] {
		t.Error("event must be marked seen")
	}
}

// Never drag an issue backwards: only a "todo" issue is started.
func TestHandleWorktreeLifecycle_DoesNotTouchIssueAlreadyStartedOrDone(t *testing.T) {
	for _, category := range []string{"in_progress", "done", "cancelled", ""} {
		t.Run("category="+category, func(t *testing.T) {
			h := newHarness()
			h.tracker.category = category

			if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
				t.Fatal(err)
			}
			if h.tracker.calls != 0 {
				t.Errorf("issue in category %q must not be transitioned", category)
			}
			if !h.processed.seen["ev-1"] {
				t.Error("event must still be marked seen")
			}
		})
	}
}

// Deleting a worktree is routine housekeeping, not a statement about the issue.
func TestHandleWorktreeLifecycle_DeletedNeverChangesTheIssue(t *testing.T) {
	for _, hadOpenPR := range []bool{false, true} {
		h := newHarness()
		ev := jiraCreated("ev-1")
		ev.Deleted, ev.HadOpenPR = true, hadOpenPR

		if err := h.uc.HandleWorktreeLifecycle(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
		if h.providerCalls() != 0 || h.tracker.catCalls != 0 {
			t.Errorf("worktree.deleted (had_open_pr=%v) must not touch the tracker; calls=%d", hadOpenPR, h.providerCalls())
		}
		if !h.processed.seen["ev-1"] {
			t.Error("event must be marked seen")
		}
	}
}

func TestHandleWorktreeLifecycle_NoActorIsSkippedNotRetried(t *testing.T) {
	h := newHarness()
	ev := jiraCreated("ev-1")
	ev.ActorUserID = ""

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if h.providerCalls() != 0 || h.tracker.catCalls != 0 || !h.processed.seen["ev-1"] {
		t.Errorf("an event with no actor cannot pick a credential; calls=%d seen=%v", h.providerCalls(), h.processed.seen["ev-1"])
	}
}

func TestHandleWorktreeLifecycle_UnsupportedProvidersAreSkipped(t *testing.T) {
	for _, provider := range []string{"linear", "github", "gitlab"} {
		h := newHarness()
		ev := jiraCreated("ev-1")
		ev.LinkedIssueProvider = provider

		if err := h.uc.HandleWorktreeLifecycle(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
		if h.providerCalls() != 0 || !h.processed.seen["ev-1"] {
			t.Errorf("%s must be skipped, got calls=%d", provider, h.providerCalls())
		}
	}
}

func TestHandleWorktreeLifecycle_RetriesTransientFailureThenSucceeds(t *testing.T) {
	h := newHarness()
	h.tracker.failN = retryAttempts - 1

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.calls != retryAttempts {
		t.Errorf("want %d attempts, got %d", retryAttempts, h.tracker.calls)
	}
	if !h.processed.seen["ev-1"] {
		t.Error("event must be marked seen on eventual success")
	}
}

func TestHandleWorktreeLifecycle_GivesUpAfterRetryAttemptsExhausted(t *testing.T) {
	h := newHarness()
	h.tracker.failN = 999

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatalf("a give-up must not surface as an error out of the handler (BR-PI-09), got %v", err)
	}
	if h.tracker.calls != retryAttempts {
		t.Errorf("want exactly %d attempts, got %d", retryAttempts, h.tracker.calls)
	}
	if !h.processed.seen["ev-1"] {
		t.Error("event must still be marked seen after giving up")
	}
}

func TestHandleWorktreeLifecycle_CategoryLookupFailureIsRetriedAndNeverTransitions(t *testing.T) {
	h := newHarness()
	h.tracker.categoryErr = errors.New("jira unreachable")

	if err := h.uc.HandleWorktreeLifecycle(context.Background(), jiraCreated("ev-1")); err != nil {
		t.Fatal(err)
	}
	if h.tracker.catCalls != retryAttempts {
		t.Errorf("want %d lookup attempts, got %d", retryAttempts, h.tracker.catCalls)
	}
	if h.tracker.calls != 0 {
		t.Error("without knowing the current state the issue must not be transitioned")
	}
}

func TestMappingTable(t *testing.T) {
	created := mapWorktreeEventToStatus(domain.WorktreeLifecycleEvent{})
	if created.TrackerState != "In Progress" || created.OnlyFromCategory != "todo" {
		t.Errorf("worktree.created -> In Progress only from todo, got %+v", created)
	}
	for _, hadOpenPR := range []bool{false, true} {
		if got := mapWorktreeEventToStatus(domain.WorktreeLifecycleEvent{Deleted: true, HadOpenPR: hadOpenPR}); got != (domain.TargetState{}) {
			t.Errorf("worktree.deleted (had_open_pr=%v) must map to nothing, got %+v", hadOpenPR, got)
		}
	}

	prCases := []struct {
		name string
		ev   domain.PullRequestLifecycleEvent
		want domain.TargetState
	}{
		{"pr.created -> In Review", domain.PullRequestLifecycleEvent{Merged: false}, domain.TargetState{TrackerState: "In Review", GitHubLabelPatch: "add:in-review", OnlyFromCategory: "in_progress"}},
		{"pr.merged -> Done", domain.PullRequestLifecycleEvent{Merged: true}, domain.TargetState{TrackerState: "Done", GitHubLabelPatch: "close", OnlyFromCategory: "in_progress"}},
	}
	for _, tc := range prCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapPullRequestEventToStatus(tc.ev); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestHandlePullRequestLifecycle_DuplicateEventIsNoOp(t *testing.T) {
	h := newHarness()
	h.processed.seen["ev-1"] = true

	err := h.uc.HandlePullRequestLifecycle(context.Background(), domain.PullRequestLifecycleEvent{
		EventID: "ev-1", LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1", Merged: true, ActorUserID: "u1",
	})
	if err != nil || h.providerCalls() != 0 {
		t.Errorf("duplicate must be a no-op; err=%v calls=%d", err, h.providerCalls())
	}
}

// An event published without an actor (older producer, or a caller with no
// user) cannot be synced; it must be skipped cleanly rather than fail three times each.
func TestHandlePullRequestLifecycle_WithoutActorIsSkipped(t *testing.T) {
	h := newHarness()

	err := h.uc.HandlePullRequestLifecycle(context.Background(), domain.PullRequestLifecycleEvent{
		EventID: "ev-1", LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1", Merged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.providerCalls() != 0 || !h.processed.seen["ev-1"] {
		t.Errorf("want skipped and marked seen; calls=%d", h.providerCalls())
	}
}

func TestHandlePullRequestLifecycle_GuardsByCurrentCategory(t *testing.T) {
	cases := []struct {
		name     string
		merged   bool
		category string
		want     string // "" means no transition
	}{
		{"created from in_progress -> In Review", false, "in_progress", "In Review"},
		{"created from todo is left alone", false, "todo", ""},
		{"created from done is left alone", false, "done", ""},
		{"merged from in_progress -> Done", true, "in_progress", "Done"},
		{"merged from todo is left alone", true, "todo", ""},
		{"merged from done is left alone", true, "done", ""},
		{"merged from unknown category is left alone", true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.tracker.category = tc.category

			err := h.uc.HandlePullRequestLifecycle(context.Background(), domain.PullRequestLifecycleEvent{
				EventID: "ev-1", TenantID: "t1", LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1",
				Merged: tc.merged, ActorUserID: "user-7",
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if h.tracker.calls != 0 {
					t.Errorf("must not transition, got state %q", h.tracker.gotState)
				}
			} else if h.tracker.calls != 1 || h.tracker.gotState != tc.want || h.tracker.gotUser != "user-7" || h.tracker.gotRef != "ENG-1" {
				t.Errorf("want one transition to %q as user-7, got calls=%d state=%q user=%q", tc.want, h.tracker.calls, h.tracker.gotState, h.tracker.gotUser)
			}
			if !h.processed.seen["ev-1"] {
				t.Error("event must be marked seen")
			}
		})
	}
}
