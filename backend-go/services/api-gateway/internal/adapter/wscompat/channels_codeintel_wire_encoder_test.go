package wscompat

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/timestamppb"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

func TestEncodeCodeIntelWire_ResultMeta(t *testing.T) {
	now := time.Date(2026, 10, 7, 5, 0, 0, 123456000, time.UTC)
	meta := &codeintelv1.ResultMeta{
		Repo:        "my-org/my-repo",
		WorktreeId:  "wt-123",
		DevServerId: "ds-secret-internal-id",
		View:        codeintelv1.ViewKind_VIEW_KIND_STRUCTURE,
		Sources: []*codeintelv1.SourceInfo{
			{
				Tool:      codeintelv1.ToolName_TOOL_NAME_GITNEXUS,
				Version:   "1.0.0",
				LineBase:  1,
				IndexedAt: "", // nullable, should be null
				Commit:    "", // nullable, should be null
			},
		},
		HeadCommit:  "", // nullable, should be null
		Stale:       false,
		Truncated:   false,
		TotalCount:  42,
		Etag:        `"abc"`,
		GeneratedAt: timestamppb.New(now),
		FromCache:   false,
		NotModified: false,
	}

	raw, err := encodeCodeIntelWire(meta)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	str := string(raw)

	// 1. devServerId must NEVER appear on wire
	if strings.Contains(str, "devServerId") || strings.Contains(str, "ds-secret-internal-id") {
		t.Fatalf("devServerId leaked to wire: %s", str)
	}

	// 2. nullable fields must be null when empty
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("invalid json generated: %v\njson: %s", err, str)
	}

	if m["headCommit"] != nil {
		t.Errorf("expected headCommit to be null, got %v", m["headCommit"])
	}

	sources, ok := m["sources"].([]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("expected 1 source, got %v", m["sources"])
	}
	src0 := sources[0].(map[string]any)
	if src0["indexedAt"] != nil {
		t.Errorf("expected source.indexedAt to be null, got %v", src0["indexedAt"])
	}
	if src0["commit"] != nil {
		t.Errorf("expected source.commit to be null, got %v", src0["commit"])
	}

	// 3. Enum ViewKind converted to lowercase
	if m["view"] != "structure" {
		t.Errorf("expected view == 'structure', got %v", m["view"])
	}

	// 4. Timestamp serialized to RFC3339 UTC
	genAt, ok := m["generatedAt"].(string)
	if !ok || !strings.HasPrefix(genAt, "2026-10-07T05:00:00") || !strings.HasSuffix(genAt, "Z") {
		t.Errorf("expected RFC3339 UTC generatedAt, got %v", genAt)
	}

	// 5. No snake_case keys allowed
	snakeRegex := regexp.MustCompile(`"[a-z]+_[a-z_]+":`)
	if match := snakeRegex.FindString(str); match != "" {
		t.Errorf("found snake_case key %s in wire output: %s", match, str)
	}
}

func TestEncodeCodeIntelWire_EnumsAndOverrides(t *testing.T) {
	// Risk enum should follow override table: uppercase
	impact := &codeintelv1.ImpactGraph{
		Risk:      codeintelv1.Risk_RISK_CRITICAL,
		Direction: "upstream",
	}

	raw, err := encodeCodeIntelWire(impact)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("failed unmarshaling json: %v", err)
	}

	if m["risk"] != "CRITICAL" {
		t.Errorf("expected Risk to be 'CRITICAL', got %v", m["risk"])
	}

	// SymbolKind should be stripped of prefix and lowercased
	sym := &codeintelv1.SymbolRef{
		Name: "myFunc",
		Kind: codeintelv1.SymbolKind_SYMBOL_KIND_FUNCTION,
	}
	rawSym, err := encodeCodeIntelWire(sym)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	var mSym map[string]any
	_ = json.Unmarshal(rawSym, &mSym)
	if mSym["kind"] != "function" {
		t.Errorf("expected kind to be 'function', got %v", mSym["kind"])
	}

	// Unknown or 0 enum becomes "unknown"
	sym0 := &codeintelv1.SymbolRef{
		Name: "unknownFunc",
		Kind: codeintelv1.SymbolKind(0),
	}
	raw0, _ := encodeCodeIntelWire(sym0)
	var m0 map[string]any
	_ = json.Unmarshal(raw0, &m0)
	if m0["kind"] != "unknown" {
		t.Errorf("expected 0 enum to be 'unknown', got %v", m0["kind"])
	}
}

func TestEncodeCodeIntelWire_EmptyCollections(t *testing.T) {
	detail := &codeintelv1.SymbolDetail{
		Flows:    []*codeintelv1.SymbolFlowRef{},
		Incoming: map[string]*codeintelv1.RelatedSymbolList{},
	}

	raw, err := encodeCodeIntelWire(detail)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	str := string(raw)
	if !strings.Contains(str, `"flows":[]`) {
		t.Errorf("expected empty array for flows: %s", str)
	}
	if !strings.Contains(str, `"incoming":{}`) {
		t.Errorf("expected empty object for incoming: %s", str)
	}
}

func TestEncodeCodeIntelWire_IntegerSafety(t *testing.T) {
	// Safe int64
	metaSafe := &codeintelv1.ResultMeta{
		TotalCount: maxSafeInt64,
	}
	_, err := encodeCodeIntelWire(metaSafe)
	if err != nil {
		t.Fatalf("expected safe int64 to succeed, got %v", err)
	}

	// Unsafe int64 exceeds 2^53 - 1
	metaUnsafe := &codeintelv1.ResultMeta{
		TotalCount: maxSafeInt64 + 1,
	}
	_, err = encodeCodeIntelWire(metaUnsafe)
	if err == nil || !strings.Contains(err.Error(), "CODEINTEL_RESULT_INVALID") {
		t.Fatalf("expected CODEINTEL_RESULT_INVALID for unsafe int64, got %v", err)
	}
}

func TestEncodeCodeIntelWire_EscapingAndUtf8(t *testing.T) {
	sym := &codeintelv1.SymbolRef{
		Name:          `<script>alert("hello");</script>`,
		QualifiedName: "func() string // utf-8: Tiếng Việt có dấu — 🚀",
	}

	raw, err := encodeCodeIntelWire(sym)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("invalid json from escaped strings: %v", err)
	}

	if m["name"] != `<script>alert("hello");</script>` {
		t.Errorf("string mismatch: %v", m["name"])
	}
	if m["qualifiedName"] != "func() string // utf-8: Tiếng Việt có dấu — 🚀" {
		t.Errorf("utf-8 mismatch: %v", m["qualifiedName"])
	}
}

func TestCodeIntelWireTables_ReferToExistingFields(t *testing.T) {
	for fullKey := range codeIntelNullableFields {
		idx := strings.LastIndex(fullKey, ".")
		if idx == -1 {
			t.Errorf("invalid nullable key format: %s", fullKey)
			continue
		}
		msgName := protoreflect.FullName(fullKey[:idx])
		jsonField := fullKey[idx+1:]

		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(msgName)
		if err != nil {
			t.Errorf("message %s not found in protoregistry: %v", msgName, err)
			continue
		}
		md, ok := desc.(protoreflect.MessageDescriptor)
		if !ok {
			t.Errorf("%s is not a MessageDescriptor", msgName)
			continue
		}
		found := false
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			if fd.JSONName() == jsonField {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("field %s not found in message %s", jsonField, msgName)
		}
	}
}

func BenchmarkEncodeCodeIntelWire(b *testing.B) {
	// Construct a synthetic 1 MiB-scale graph
	graph := &codeintelv1.ModuleGraph{
		Nodes: make([]*codeintelv1.ModuleNode, 5000),
		Edges: make([]*codeintelv1.ModuleEdge, 10000),
	}
	for i := range graph.Nodes {
		graph.Nodes[i] = &codeintelv1.ModuleNode{
			Id:          "node-id",
			Language:    "go",
			SymbolCount: 25,
			Loc:         120,
			Cluster:     "auth",
			Area:        "backend",
		}
	}
	for i := range graph.Edges {
		graph.Edges[i] = &codeintelv1.ModuleEdge{
			From:  "node-a",
			To:    "node-b",
			Count: 5,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := encodeCodeIntelWire(graph)
		if err != nil {
			b.Fatal(err)
		}
	}
}
