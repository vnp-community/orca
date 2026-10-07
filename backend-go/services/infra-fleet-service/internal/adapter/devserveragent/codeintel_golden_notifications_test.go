package devserveragent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCodeIntelGoldenNotifications(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	fixtureDir := filepath.Join("testdata", "codeintel_notifications")

	// 1. IndexChanged
	raw1, err := os.ReadFile(filepath.Join(fixtureDir, "index_changed.json"))
	if err != nil {
		t.Fatalf("reading index_changed.json: %v", err)
	}
	var notif1 JSONRPCNotification
	if err := json.Unmarshal(raw1, &notif1); err != nil {
		t.Fatalf("unmarshaling notif1: %v", err)
	}
	ev1, ok1 := decodeCodeIntelNotification(notif1, now)
	if !ok1 {
		t.Fatal("expected decode to succeed for index_changed")
	}
	if ev1.Kind != domain.CodeIntelEventKindIndexChanged {
		t.Errorf("kind = %q, want index_changed", ev1.Kind)
	}
	if ev1.WorkspaceRoot != "/Users/developer/project" || ev1.Tool != "gitnexus" || ev1.Commit != "a1b2c3d4e5f6" {
		t.Errorf("unexpected ev1 fields: %+v", ev1)
	}
	if ev1.Reason != "head_changed" || ev1.HeadCommit != "f6e5d4c3b2a1" || !ev1.Stale {
		t.Errorf("unexpected ev1 reason/headCommit/stale: %+v", ev1)
	}

	// 2. ReindexProgress
	raw2, err := os.ReadFile(filepath.Join(fixtureDir, "reindex_progress.json"))
	if err != nil {
		t.Fatalf("reading reindex_progress.json: %v", err)
	}
	var notif2 JSONRPCNotification
	if err := json.Unmarshal(raw2, &notif2); err != nil {
		t.Fatalf("unmarshaling notif2: %v", err)
	}
	ev2, ok2 := decodeCodeIntelNotification(notif2, now)
	if !ok2 {
		t.Fatal("expected decode to succeed for reindex_progress")
	}
	if ev2.Kind != domain.CodeIntelEventKindReindexProgress {
		t.Errorf("kind = %q, want reindex_progress", ev2.Kind)
	}
	if ev2.JobID != "reindex-job-42" || ev2.Tool != "codegraph" || ev2.Stage != "parsing_ast" {
		t.Errorf("unexpected ev2 fields: %+v", ev2)
	}
	if ev2.Percent == nil || *ev2.Percent != 85 {
		t.Errorf("percent = %v, want 85", ev2.Percent)
	}

	// 3. QualityProgress
	raw3, err := os.ReadFile(filepath.Join(fixtureDir, "quality_progress.json"))
	if err != nil {
		t.Fatalf("reading quality_progress.json: %v", err)
	}
	var notif3 JSONRPCNotification
	if err := json.Unmarshal(raw3, &notif3); err != nil {
		t.Fatalf("unmarshaling notif3: %v", err)
	}
	ev3, ok3 := decodeCodeIntelNotification(notif3, now)
	if !ok3 {
		t.Fatal("expected decode to succeed for quality_progress")
	}
	if ev3.Kind != domain.CodeIntelEventKindQualityProgress {
		t.Errorf("kind = %q, want quality_progress", ev3.Kind)
	}
	if ev3.RunID != "qual-run-99" || ev3.Stage != "linting" || ev3.Percent == nil || *ev3.Percent != 45 {
		t.Errorf("unexpected ev3 fields: %+v", ev3)
	}

	// 4. QualityFinished
	raw4, err := os.ReadFile(filepath.Join(fixtureDir, "quality_finished.json"))
	if err != nil {
		t.Fatalf("reading quality_finished.json: %v", err)
	}
	var notif4 JSONRPCNotification
	if err := json.Unmarshal(raw4, &notif4); err != nil {
		t.Fatalf("unmarshaling notif4: %v", err)
	}
	ev4, ok4 := decodeCodeIntelNotification(notif4, now)
	if !ok4 {
		t.Fatal("expected decode to succeed for quality_finished")
	}
	if ev4.Kind != domain.CodeIntelEventKindQualityFinished {
		t.Errorf("kind = %q, want quality_finished", ev4.Kind)
	}
	if ev4.RunID != "qual-run-99" || ev4.HeadCommit != "f6e5d4c3b2a1" {
		t.Errorf("unexpected ev4 fields: %+v", ev4)
	}
}

func TestProtoReflection_CodeIntelEvent(t *testing.T) {
	msg := &infrafleetv1.CodeIntelEvent{}
	desc := msg.ProtoReflect().Descriptor()
	fields := desc.Fields()

	if fields.Len() != 20 {
		t.Fatalf("expected exactly 20 fields in CodeIntelEvent proto, got %d", fields.Len())
	}

	expectedFieldNames := map[int]string{
		1:  "kind",
		2:  "workspace_root",
		3:  "tool",
		4:  "commit",
		5:  "indexed_at",
		6:  "job_id",
		7:  "stage",
		8:  "percent",
		9:  "message",
		10: "received_at",
		11: "reason",
		12: "head_commit",
		13: "stale",
		14: "index_scope",
		15: "merge_base",
		16: "trigger",
		17: "outcome",
		18: "error_code",
		19: "run_id",
		20: "payload_json",
	}

	for num, wantName := range expectedFieldNames {
		f := fields.ByNumber(protoreflect.FieldNumber(num))
		if f == nil {
			t.Fatalf("missing field with number %d (want %q)", num, wantName)
		}
		if string(f.Name()) != wantName {
			t.Errorf("field %d: got name %q, want %q", num, f.Name(), wantName)
		}
	}

	// Field 8 (percent) must have presence tracking (optional)
	f8 := fields.ByNumber(protoreflect.FieldNumber(8))
	if !f8.HasPresence() {
		t.Errorf("field 8 (percent) must have presence tracking (optional)")
	}
}
