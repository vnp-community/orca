package wscompat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCodeIntelEnvelope(t *testing.T) {
	t.Run("full meta fields with null fallbacks", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 14, 30, 0, 0, time.UTC)
		meta := &codeintelv1.ResultMeta{
			Repo:        "orca",
			WorktreeId:  "wt-123",
			DevServerId: "should-not-leak",
			View:        codeintelv1.ViewKind_VIEW_KIND_STRUCTURE,
			Sources: []*codeintelv1.SourceInfo{
				{
					Tool:      codeintelv1.ToolName_TOOL_NAME_GITNEXUS,
					Version:   "1.2.3",
					IndexedAt: "",
					Commit:    "",
					LineBase:  1,
				},
			},
			HeadCommit:  "",
			Stale:       false,
			Truncated:   false,
			TotalCount:  0, // Must still appear
			Etag:        `"etag-xyz"`,
			FromCache:   true,
			GeneratedAt: timestamppb.New(now),
		}

		data := &codeintelv1.SymbolDetail{
			Symbol: &codeintelv1.SymbolRef{
				Name: "HandleRequest",
			},
		}

		raw, err := encodeEnvelope(meta, data, "next-tok-1")
		if err != nil {
			t.Fatalf("encodeEnvelope failed: %v", err)
		}

		str := string(raw)
		if strings.Contains(str, "should-not-leak") {
			t.Fatalf("devServerId leaked in envelope: %s", str)
		}

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("json parse failed: %v", err)
		}

		if parsed["totalCount"] != float64(0) {
			t.Fatalf("expected totalCount: 0, got %v", parsed["totalCount"])
		}
		if parsed["headCommit"] != nil {
			t.Fatalf("expected headCommit: null, got %v", parsed["headCommit"])
		}
		if parsed["nextPageToken"] != "next-tok-1" {
			t.Fatalf("expected nextPageToken: next-tok-1, got %v", parsed["nextPageToken"])
		}
		if parsed["view"] != "structure" {
			t.Fatalf("expected view: structure, got %v", parsed["view"])
		}
		if parsed["data"] == nil {
			t.Fatalf("expected data object, got nil")
		}

		sources := parsed["sources"].([]any)
		src0 := sources[0].(map[string]any)
		if src0["indexedAt"] != nil || src0["commit"] != nil {
			t.Fatalf("expected indexedAt and commit to be null in sources: %v", src0)
		}
		if src0["lineBase"] != float64(1) {
			t.Fatalf("expected lineBase 1, got %v", src0["lineBase"])
		}
	})

	t.Run("notModified omits data and sets notModified: true", func(t *testing.T) {
		meta := &codeintelv1.ResultMeta{
			WorktreeId:  "wt-123",
			View:        codeintelv1.ViewKind_VIEW_KIND_STRUCTURE,
			NotModified: true,
			Etag:        `"etag-123"`,
		}
		data := &codeintelv1.SymbolDetail{
			Symbol: &codeintelv1.SymbolRef{Name: "Foo"},
		}

		raw, err := encodeEnvelope(meta, data, "")
		if err != nil {
			t.Fatalf("encodeEnvelope failed: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("json parse failed: %v", err)
		}

		if parsed["notModified"] != true {
			t.Fatalf("expected notModified: true, got %v", parsed["notModified"])
		}
		if _, hasData := parsed["data"]; hasData {
			t.Fatalf("expected no data key when notModified, got %v", parsed["data"])
		}
	})
}
