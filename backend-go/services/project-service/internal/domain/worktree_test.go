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
			_, err := NewWorktree("w1", tt.projectID, tt.repoID, tt.path, tt.branch, WorktreeLineageCapture{})
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
	wt, err := NewWorktree("w1", "p1", "r1", "/srv/worktrees/w1", "main", WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.Active {
		t.Error("expected a freshly recorded worktree to start active")
	}
}

func TestNewWorktree_StampsExplicitCaptureConfidenceWhenLineageSupplied(t *testing.T) {
	wt, err := NewWorktree("w2", "p1", "r1", "/srv/worktrees/w2", "feature", WorktreeLineageCapture{
		ParentWorktreeID: "w1", Origin: "cli",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.CaptureConfidence == nil || *wt.CaptureConfidence != "explicit" {
		t.Fatalf("expected CaptureConfidence to be stamped \"explicit\", got %v", wt.CaptureConfidence)
	}
	if wt.ParentWorktreeID == nil || *wt.ParentWorktreeID != "w1" {
		t.Fatalf("expected ParentWorktreeID to round-trip, got %v", wt.ParentWorktreeID)
	}
}

func TestNewWorktree_NoLineageMeansNoCaptureConfidence(t *testing.T) {
	wt, err := NewWorktree("w1", "p1", "r1", "/srv/worktrees/w1", "main", WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.CaptureConfidence != nil {
		t.Fatalf("expected no CaptureConfidence when no lineage was supplied, got %v", *wt.CaptureConfidence)
	}
}
