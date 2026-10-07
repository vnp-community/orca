package usecase

import (
	"testing"
)

func TestDeterministicEventID_StableAndUnique(t *testing.T) {
	tools := []string{"gitnexus", "codegraph"}
	id1 := DeterministicEventID("tenant-1", "binding-1", tools, "commit-a", "2026-10-06T12:00:00Z", "changed")
	id2 := DeterministicEventID("tenant-1", "binding-1", tools, "commit-a", "2026-10-06T12:00:00Z", "changed")

	if id1 != id2 {
		t.Fatalf("expected identical IDs for same inputs, got %s vs %s", id1, id2)
	}

	id3 := DeterministicEventID("tenant-1", "binding-1", tools, "commit-b", "2026-10-06T12:00:00Z", "changed")
	if id1 == id3 {
		t.Fatalf("expected different IDs for different commit, got same: %s", id1)
	}
}
