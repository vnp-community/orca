package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/domain"
)

// fakeTracker/fakeScm/fakeProjects/fakeProcessedEvents are in-memory fakes —
// the "test against fakes, not a real gRPC client" pattern this codebase's
// usecase tests already establish (see scm-integration-service's
// scm_provider_dispatch_test.go).

type fakeTracker struct {
	calls   int
	failN   int // fail the first failN calls, then succeed
	lastErr error
}

func (f *fakeTracker) TransitionIssue(ctx context.Context, tenantID, provider, ref, state string) error {
	f.calls++
	if f.calls <= f.failN {
		f.lastErr = errors.New("transient failure")
		return f.lastErr
	}
	return nil
}

type fakeScm struct {
	calls int
	failN int
}

func (f *fakeScm) UpdateIssue(ctx context.Context, tenantID, provider, ref, labelPatch string) error {
	f.calls++
	if f.calls <= f.failN {
		return errors.New("transient failure")
	}
	return nil
}

func (f *fakeScm) GetPullRequestForBranch(ctx context.Context, tenantID, provider, repo, branch string) (bool, error) {
	return false, nil
}

type fakeProjects struct {
	enabled bool
	err     error
	calls   int
}

func (f *fakeProjects) IsIssueStatusSyncEnabled(ctx context.Context, tenantID, projectID string) (bool, error) {
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

func (f *fakeProcessedEvents) Seen(ctx context.Context, eventID string) (bool, error) {
	return f.seen[eventID], nil
}

func (f *fakeProcessedEvents) MarkSeen(ctx context.Context, eventID string) error {
	f.seen[eventID] = true
	f.marked = append(f.marked, eventID)
	return nil
}

func TestHandleWorktreeLifecycle_DuplicateEventIsNoOp(t *testing.T) {
	tracker := &fakeTracker{}
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: true}
	processed := newFakeProcessedEvents()
	processed.seen["ev-1"] = true
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandleWorktreeLifecycle(context.Background(), domain.WorktreeLifecycleEvent{
		EventID: "ev-1", ProjectID: "p1", LinkedIssueProvider: "github", LinkedIssueRef: "owner/repo#1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.calls != 0 || scm.calls != 0 {
		t.Errorf("expected zero provider calls for a duplicate event, got tracker=%d scm=%d", tracker.calls, scm.calls)
	}
}

func TestHandleWorktreeLifecycle_EmptyLinkedIssueMarksSeenWithoutCalling(t *testing.T) {
	tracker := &fakeTracker{}
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: true}
	processed := newFakeProcessedEvents()
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandleWorktreeLifecycle(context.Background(), domain.WorktreeLifecycleEvent{
		EventID: "ev-1", ProjectID: "p1", LinkedIssueProvider: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.calls != 0 || scm.calls != 0 {
		t.Errorf("expected zero provider calls when linked_issue_provider is empty, got tracker=%d scm=%d", tracker.calls, scm.calls)
	}
	if !processed.seen["ev-1"] {
		t.Error("expected event to be marked seen")
	}
}

func TestHandleWorktreeLifecycle_SyncDisabledMarksSeenWithoutCalling(t *testing.T) {
	tracker := &fakeTracker{}
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: false} // BR-PI-07 re-check: flag flipped off mid-flight
	processed := newFakeProcessedEvents()
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandleWorktreeLifecycle(context.Background(), domain.WorktreeLifecycleEvent{
		EventID: "ev-1", ProjectID: "p1", LinkedIssueProvider: "github", LinkedIssueRef: "owner/repo#1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.calls != 0 || scm.calls != 0 {
		t.Errorf("expected zero provider calls when sync is disabled, got tracker=%d scm=%d", tracker.calls, scm.calls)
	}
	if !processed.seen["ev-1"] {
		t.Error("expected event to be marked seen")
	}
}

func TestHandleWorktreeLifecycle_RetriesThenSucceeds(t *testing.T) {
	tracker := &fakeTracker{failN: 2} // fails twice, succeeds on the 3rd
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: true}
	processed := newFakeProcessedEvents()
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandleWorktreeLifecycle(context.Background(), domain.WorktreeLifecycleEvent{
		EventID: "ev-1", ProjectID: "p1", LinkedIssueProvider: "linear", LinkedIssueRef: "ENG-1", Deleted: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.calls != retryAttempts {
		t.Errorf("expected exactly %d attempts, got %d", retryAttempts, tracker.calls)
	}
	if !processed.seen["ev-1"] {
		t.Error("expected event to be marked seen on eventual success")
	}
}

func TestHandleWorktreeLifecycle_GivesUpAfterRetryAttemptsExhausted(t *testing.T) {
	tracker := &fakeTracker{failN: 999} // always fails
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: true}
	processed := newFakeProcessedEvents()
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandleWorktreeLifecycle(context.Background(), domain.WorktreeLifecycleEvent{
		EventID: "ev-1", ProjectID: "p1", LinkedIssueProvider: "jira", LinkedIssueRef: "PROJ-1", Deleted: false,
	})
	if err != nil {
		t.Fatalf("expected no error out of HandleWorktreeLifecycle even on give-up (BR-PI-09), got: %v", err)
	}
	if tracker.calls != retryAttempts {
		t.Errorf("expected exactly %d attempts (not 4, not unbounded), got %d", retryAttempts, tracker.calls)
	}
	if !processed.seen["ev-1"] {
		t.Error("expected event to still be marked seen after give-up")
	}
}

func TestMappingTable(t *testing.T) {
	cases := []struct {
		name string
		ev   domain.WorktreeLifecycleEvent
		want domain.TargetState
	}{
		{
			name: "worktree.created -> In Progress",
			ev:   domain.WorktreeLifecycleEvent{Deleted: false},
			want: domain.TargetState{TrackerState: "In Progress", GitHubLabelPatch: "add:in-progress"},
		},
		{
			name: "worktree.deleted && !had_open_pr -> Cancelled",
			ev:   domain.WorktreeLifecycleEvent{Deleted: true, HadOpenPR: false},
			want: domain.TargetState{TrackerState: "Cancelled", GitHubLabelPatch: "close"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapWorktreeEventToStatus(tc.ev)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	prCases := []struct {
		name string
		ev   domain.PullRequestLifecycleEvent
		want domain.TargetState
	}{
		{
			name: "pr.created -> In Review",
			ev:   domain.PullRequestLifecycleEvent{Merged: false},
			want: domain.TargetState{TrackerState: "In Review", GitHubLabelPatch: "add:in-review"},
		},
		{
			name: "pr.merged -> Done",
			ev:   domain.PullRequestLifecycleEvent{Merged: true},
			want: domain.TargetState{TrackerState: "Done", GitHubLabelPatch: "close"},
		},
	}
	for _, tc := range prCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapPullRequestEventToStatus(tc.ev)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestHandlePullRequestLifecycle_DuplicateEventIsNoOp(t *testing.T) {
	tracker := &fakeTracker{}
	scm := &fakeScm{}
	projects := &fakeProjects{enabled: true}
	processed := newFakeProcessedEvents()
	processed.seen["ev-1"] = true
	uc := NewSyncIssueStatus(tracker, scm, projects, processed, nil)

	err := uc.HandlePullRequestLifecycle(context.Background(), domain.PullRequestLifecycleEvent{
		EventID: "ev-1", LinkedIssueProvider: "github", LinkedIssueRef: "owner/repo#1", Merged: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracker.calls != 0 || scm.calls != 0 {
		t.Errorf("expected zero provider calls for a duplicate event, got tracker=%d scm=%d", tracker.calls, scm.calls)
	}
}
