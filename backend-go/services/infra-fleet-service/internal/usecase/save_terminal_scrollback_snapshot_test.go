package usecase

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSaveTerminalScrollbackSnapshot_HappyPath_CompressesAndUpserts(t *testing.T) {
	repo := &fakeTerminalScrollbackSnapshotRepository{}
	clock := fakeClock{now: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)}
	uc := NewSaveTerminalScrollbackSnapshot(repo, clock)

	ctx := withTenant(context.Background(), "tenant-1")
	original := []byte("line one\r\nline two\r\n")
	err := uc.Execute(ctx, SaveTerminalScrollbackSnapshotInput{
		WorktreeID: "wt-1", PaneKey: "pane-1", Cols: 80, Rows: 24, Data: original, LastTitle: "bash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.upsertCalls) != 1 {
		t.Fatalf("expected exactly one Upsert call, got %d", len(repo.upsertCalls))
	}
	snap := repo.upsertCalls[0]
	if snap.TenantID != "tenant-1" || snap.WorktreeID != "wt-1" || snap.PaneKey != "pane-1" {
		t.Errorf("unexpected snapshot key: %+v", snap)
	}
	if snap.UncompressedBytes != int64(len(original)) {
		t.Errorf("expected UncompressedBytes %d, got %d", len(original), snap.UncompressedBytes)
	}

	r, err := gzip.NewReader(bytes.NewReader(snap.DataGzip))
	if err != nil {
		t.Fatalf("DataGzip is not valid gzip: %v", err)
	}
	decompressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Errorf("decompressed data does not round-trip: got %q, want %q", decompressed, original)
	}
}

func TestSaveTerminalScrollbackSnapshot_OverCap_RejectedWithoutUpsert(t *testing.T) {
	repo := &fakeTerminalScrollbackSnapshotRepository{
		sumBytesByWorktree: map[string]int64{"tenant-1/wt-1": domain.MaxSnapshotBytesPerWorktree},
	}
	clock := fakeClock{now: time.Now()}
	uc := NewSaveTerminalScrollbackSnapshot(repo, clock)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, SaveTerminalScrollbackSnapshotInput{
		WorktreeID: "wt-1", PaneKey: "pane-1", Data: []byte("x"),
	})
	if err == nil {
		t.Fatal("expected an over-cap error")
	}
	if len(repo.upsertCalls) != 0 {
		t.Errorf("expected no Upsert call when over cap, got %d", len(repo.upsertCalls))
	}
}

func TestSaveTerminalScrollbackSnapshot_MissingKey_RejectedBeforeTouchingRepository(t *testing.T) {
	repo := &fakeTerminalScrollbackSnapshotRepository{}
	uc := NewSaveTerminalScrollbackSnapshot(repo, fakeClock{now: time.Now()})

	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, SaveTerminalScrollbackSnapshotInput{WorktreeID: "", PaneKey: "pane-1", Data: []byte("x")}); err == nil {
		t.Error("expected an error for missing WorktreeID")
	}
	if err := uc.Execute(ctx, SaveTerminalScrollbackSnapshotInput{WorktreeID: "wt-1", PaneKey: "", Data: []byte("x")}); err == nil {
		t.Error("expected an error for missing PaneKey")
	}
	if len(repo.upsertCalls) != 0 {
		t.Errorf("expected no Upsert call for missing-key inputs, got %d", len(repo.upsertCalls))
	}
}
