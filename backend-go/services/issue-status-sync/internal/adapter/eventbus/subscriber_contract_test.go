package eventbus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

type recordingTracker struct {
	user, ref, state string
	transitions      int
}

func (r *recordingTracker) TransitionIssue(_ context.Context, _, userID, _, ref, state string) error {
	r.transitions++
	r.user, r.ref, r.state = userID, ref, state
	return nil
}
func (r *recordingTracker) IssueStatusCategory(context.Context, string, string, string, string) (string, error) {
	return "todo", nil
}

type noScm struct{}

func (noScm) UpdateIssue(context.Context, string, string, string, string) error { return nil }
func (noScm) GetPullRequestForBranch(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

type syncOn struct{}

func (syncOn) IsIssueStatusSyncEnabled(context.Context, string, string) (bool, error) {
	return true, nil
}

type memSeen map[string]bool

func (m memSeen) Seen(_ context.Context, id string) (bool, error) { return m[id], nil }
func (m memSeen) MarkSeen(_ context.Context, id string) error     { m[id] = true; return nil }

func newContractSubscriber() (*Subscriber, *recordingTracker) {
	tracker := &recordingTracker{}
	return New(nil, usecase.NewSyncIssueStatus(tracker, noScm{}, syncOn{}, memSeen{}, nil), nil), tracker
}

func event(payload string) commoneventbus.Event {
	return commoneventbus.Event{ID: "ev-1", TenantID: "t1", OccurredAt: time.Now(), Version: 1, Payload: json.RawMessage(payload)}
}

// The payload below is exactly what project-service's worktreeLifecycleEventPayload
// marshals; if either side renames a field this fails instead of the sync
// silently doing nothing.
func TestWorktreeCreatedPayloadFromProjectServiceDrivesAJiraTransition(t *testing.T) {
	sub, tracker := newContractSubscriber()
	payload := `{"worktree_id":"w1","project_id":"p1","linked_issue_provider":"jira","linked_issue_ref":"ENG-1","had_open_pr":false,"actor_user_id":"user-7"}`

	if err := sub.handleWorktreeEvent(false)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	if tracker.transitions != 1 || tracker.ref != "ENG-1" || tracker.state != "In Progress" || tracker.user != "user-7" {
		t.Errorf("want In Progress for ENG-1 as user-7, got %+v", tracker)
	}
}

func TestEventPublishedBeforeActorExistedIsSkippedNotFailed(t *testing.T) {
	sub, tracker := newContractSubscriber()
	payload := `{"worktree_id":"w1","project_id":"p1","linked_issue_provider":"jira","linked_issue_ref":"ENG-1","had_open_pr":false}`

	if err := sub.handleWorktreeEvent(false)(context.Background(), event(payload)); err != nil {
		t.Fatalf("an old event must be acknowledged, not redelivered forever: %v", err)
	}
	if tracker.transitions != 0 {
		t.Error("without an actor nothing may be sent to Jira")
	}
}

func TestWorktreeDeletedPayloadNeverTouchesJira(t *testing.T) {
	sub, tracker := newContractSubscriber()
	payload := `{"worktree_id":"w1","project_id":"p1","linked_issue_provider":"jira","linked_issue_ref":"ENG-1","had_open_pr":false,"actor_user_id":"user-7"}`

	if err := sub.handleWorktreeEvent(true)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	if tracker.transitions != 0 {
		t.Error("deleting a worktree must never change the issue")
	}
}

func TestMalformedPayloadIsAcknowledged(t *testing.T) {
	sub, tracker := newContractSubscriber()
	if err := sub.handleWorktreeEvent(false)(context.Background(), event(`{not json`)); err != nil {
		t.Fatalf("a malformed payload would redeliver forever; it must be dropped, got %v", err)
	}
	if tracker.transitions != 0 {
		t.Error("nothing may be sent")
	}
}
