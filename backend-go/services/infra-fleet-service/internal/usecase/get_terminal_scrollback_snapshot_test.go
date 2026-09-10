package usecase

import (
	"context"
	"testing"
	"time"
)

func TestGetTerminalScrollbackSnapshot_NotFound_ReturnsFoundFalseNoError(t *testing.T) {
	repo := &fakeTerminalScrollbackSnapshotRepository{}
	uc := NewGetTerminalScrollbackSnapshot(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, "wt-never-saved", "pane-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Found {
		t.Error("expected Found=false for a never-saved pane")
	}
}

func TestGetTerminalScrollbackSnapshot_RoundTripsExactBytes(t *testing.T) {
	saveRepo := &fakeTerminalScrollbackSnapshotRepository{}
	clock := fakeClock{now: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)}
	saveUC := NewSaveTerminalScrollbackSnapshot(saveRepo, clock)

	ctx := withTenant(context.Background(), "tenant-1")
	original := []byte("\x1b[31mred text\x1b[0m\r\nmore lines\r\n")
	if err := saveUC.Execute(ctx, SaveTerminalScrollbackSnapshotInput{
		WorktreeID: "wt-1", PaneKey: "pane-1", Cols: 80, Rows: 24, Data: original, LastTitle: "zsh",
	}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	getUC := NewGetTerminalScrollbackSnapshot(saveRepo)
	result, err := getUC.Execute(ctx, "wt-1", "pane-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Fatal("expected Found=true for a saved snapshot")
	}
	if string(result.Data) != string(original) {
		t.Errorf("decompressed round-trip mismatch: got %q, want %q", result.Data, original)
	}
	if result.LastTitle != "zsh" {
		t.Errorf("expected LastTitle %q, got %q", "zsh", result.LastTitle)
	}
}

func TestGetTerminalScrollbackSnapshot_RequiresTenantContext(t *testing.T) {
	uc := NewGetTerminalScrollbackSnapshot(&fakeTerminalScrollbackSnapshotRepository{})
	_, err := uc.Execute(context.Background(), "wt-1", "pane-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
