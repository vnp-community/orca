package domain

import "testing"

func TestNewTask_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		title    string
		status   string
		parentID string
		id       string
		wantErr  error
	}{
		{"valid", "t1", "Title", StatusOpen, "", "task-1", nil},
		{"defaults status when empty", "t1", "Title", "", "", "task-1", nil},
		{"empty tenant", "", "Title", StatusOpen, "", "task-1", ErrEmptyTenant},
		{"empty title", "t1", "", StatusOpen, "", "task-1", ErrEmptyTitle},
		{"invalid status", "t1", "Title", "bogus", "", "task-1", ErrInvalidStatus},
		{"self parent", "t1", "Title", StatusOpen, "task-1", "task-1", ErrSelfParent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTask(tt.id, tt.tenantID, tt.title, tt.status, tt.parentID, "")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if got.Status == "" {
					t.Errorf("expected a default status to be applied")
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestTask_SetStatus_RejectsTransitionOutOfTerminalState(t *testing.T) {
	done, err := NewTask("t1", "tenant-1", "Title", StatusDone, "", "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	if _, err := done.SetStatus(StatusOpen); err != ErrTerminalStatus {
		t.Fatalf("expected ErrTerminalStatus, got %v", err)
	}
}

func TestTask_SetStatus_AllowsNonTerminalTransition(t *testing.T) {
	open, err := NewTask("t1", "tenant-1", "Title", StatusOpen, "", "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	done, err := open.SetStatus(StatusDone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done.Status != StatusDone {
		t.Errorf("expected status %q, got %q", StatusDone, done.Status)
	}
}

// TestTask_SetStatus_RejectsTransitionIntoInProgress locks in TASK-223's
// guard: SetStatus (the method UpdateTask calls) must never be able to mark
// a task in_progress — only ExecuteTask's direct
// TaskRepository.UpdateStatus call may do that. See ErrCannotSetInProgress's
// doc comment.
func TestTask_SetStatus_RejectsTransitionIntoInProgress(t *testing.T) {
	open, err := NewTask("t1", "tenant-1", "Title", StatusOpen, "", "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	if _, err := open.SetStatus(StatusInProgress); err != ErrCannotSetInProgress {
		t.Fatalf("expected ErrCannotSetInProgress, got %v", err)
	}
}

// TestTask_SetStatus_AllowsBlockedAndReview locks in TASK-TG-01-03's new
// status values participating in the same permissive transition matrix as
// every other non-terminal, non-in_progress status.
func TestTask_SetStatus_AllowsBlockedAndReview(t *testing.T) {
	open, err := NewTask("t1", "tenant-1", "Title", StatusOpen, "", "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	blocked, err := open.SetStatus(StatusBlocked)
	if err != nil {
		t.Fatalf("unexpected error transitioning to blocked: %v", err)
	}
	if blocked.Status != StatusBlocked {
		t.Errorf("expected status %q, got %q", StatusBlocked, blocked.Status)
	}

	review, err := blocked.SetStatus(StatusReview)
	if err != nil {
		t.Fatalf("unexpected error transitioning to review: %v", err)
	}
	if review.Status != StatusReview {
		t.Errorf("expected status %q, got %q", StatusReview, review.Status)
	}
}

func TestNewTask_AcceptsBlockedAndReviewAsInitialStatus(t *testing.T) {
	if _, err := NewTask("t1", "tenant-1", "Title", StatusBlocked, "", ""); err != nil {
		t.Fatalf("unexpected error with initial status blocked: %v", err)
	}
	if _, err := NewTask("t1", "tenant-1", "Title", StatusReview, "", ""); err != nil {
		t.Fatalf("unexpected error with initial status review: %v", err)
	}
}
