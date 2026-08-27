package domain

import "testing"

func TestNewWorktree_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		repoID    string
		path      string
		branch    string
		wantErr   error
	}{
		{"valid", "p1", "r1", "/srv/worktrees/w1", "main", nil},
		{"empty project id", "", "r1", "/srv/worktrees/w1", "main", ErrEmptyProjectID},
		{"empty repo id", "p1", "", "/srv/worktrees/w1", "main", ErrEmptyRepoID},
		{"empty path", "p1", "r1", "", "main", ErrEmptyWorktreePath},
		{"empty branch", "p1", "r1", "/srv/worktrees/w1", "", ErrEmptyWorktreeBranch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWorktree("w1", tt.projectID, tt.repoID, tt.path, tt.branch, "")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewWorktree_StartsActive(t *testing.T) {
	wt, err := NewWorktree("w1", "p1", "r1", "/srv/worktrees/w1", "main", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.Active {
		t.Error("expected a freshly recorded worktree to start active")
	}
	if wt.Status != WorktreeStatusActive {
		t.Errorf("expected a freshly recorded worktree to start with status=active, got %v", wt.Status)
	}
}

func TestNewWorktree_IdempotencyKey_EmptyIsNil(t *testing.T) {
	wt, err := NewWorktree("w1", "p1", "r1", "/srv/worktrees/w1", "main", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.IdempotencyKey != nil {
		t.Errorf("expected nil IdempotencyKey for empty input, got %v", *wt.IdempotencyKey)
	}
}

func TestNewWorktree_IdempotencyKey_NonEmptyIsSet(t *testing.T) {
	wt, err := NewWorktree("w1", "p1", "r1", "/srv/worktrees/w1", "main", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.IdempotencyKey == nil || *wt.IdempotencyKey != "abc123" {
		t.Errorf("expected IdempotencyKey=abc123, got %v", wt.IdempotencyKey)
	}
}
