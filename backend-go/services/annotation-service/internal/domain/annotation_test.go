package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewAnchor(t *testing.T) {
	t.Run("rejects EndLine < Line", func(t *testing.T) {
		_, err := NewAnchor("repo", "", "file.go", 10, 5, SideUnspecified, "main")
		if !errors.Is(err, ErrEndLineBeforeLine) {
			t.Fatalf("expected ErrEndLineBeforeLine, got %v", err)
		}
	})

	t.Run("accepts SideUnspecified", func(t *testing.T) {
		a, err := NewAnchor("repo", "", "file.go", 10, 0, SideUnspecified, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Side != SideUnspecified {
			t.Fatalf("expected SideUnspecified, got %v", a.Side)
		}
	})

	t.Run("accepts EndLine == Line", func(t *testing.T) {
		a, err := NewAnchor("repo", "", "file.go", 10, 10, SideNew, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.EndLine != 10 {
			t.Fatalf("expected EndLine=10, got %d", a.EndLine)
		}
	})

	t.Run("accepts EndLine > Line", func(t *testing.T) {
		a, err := NewAnchor("repo", "worktree-1", "file.go", 10, 20, SideOld, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.EndLine != 20 || a.WorktreeID != "worktree-1" {
			t.Fatalf("unexpected anchor: %+v", a)
		}
	})
}

func TestAnnotation_MarkSent(t *testing.T) {
	anchor, err := NewAnchor("repo", "", "file.go", 1, 0, SideUnspecified, "main")
	if err != nil {
		t.Fatalf("unexpected error building anchor: %v", err)
	}
	now := time.Now().UTC()
	original, err := NewAnnotation("id-1", "tenant-1", "author-1", anchor, "content", "code", false, "req-1", now, now)
	if err != nil {
		t.Fatalf("unexpected error building annotation: %v", err)
	}

	sentAt := now.Add(time.Minute)
	updated := original.MarkSent(sentAt)

	if !updated.SentToAgent {
		t.Fatalf("expected SentToAgent=true")
	}
	if updated.SentAt == nil || !updated.SentAt.Equal(sentAt) {
		t.Fatalf("expected SentAt=%v, got %v", sentAt, updated.SentAt)
	}

	// Copy semantics — the receiver must not be mutated.
	if original.SentToAgent {
		t.Fatalf("expected original.SentToAgent to remain false")
	}
	if original.SentAt != nil {
		t.Fatalf("expected original.SentAt to remain nil")
	}

	// Rest of the struct unchanged.
	updated.SentToAgent = false
	updated.SentAt = nil
	if updated != original {
		t.Fatalf("expected only SentToAgent/SentAt to change, got diff:\n%+v\nvs\n%+v", updated, original)
	}
}
