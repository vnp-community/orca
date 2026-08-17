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
			got, err := NewTask(tt.id, tt.tenantID, tt.title, tt.status, tt.parentID)
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
	done, err := NewTask("t1", "tenant-1", "Title", StatusDone, "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	if _, err := done.SetStatus(StatusOpen); err != ErrTerminalStatus {
		t.Fatalf("expected ErrTerminalStatus, got %v", err)
	}
}

func TestTask_SetStatus_AllowsNonTerminalTransition(t *testing.T) {
	open, err := NewTask("t1", "tenant-1", "Title", StatusOpen, "")
	if err != nil {
		t.Fatalf("unexpected error building task: %v", err)
	}

	inProgress, err := open.SetStatus(StatusInProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inProgress.Status != StatusInProgress {
		t.Errorf("expected status %q, got %q", StatusInProgress, inProgress.Status)
	}
}
