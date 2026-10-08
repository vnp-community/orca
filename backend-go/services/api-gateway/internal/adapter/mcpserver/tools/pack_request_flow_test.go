package tools

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

func requestFlowSpecs() []*ToolSpec {
	var out []*ToolSpec
	for _, s := range AllSpecs() {
		if isRequestFlowNamespace(s.Namespace) {
			out = append(out, s)
		}
	}
	return out
}

func TestRequestFlowPack_CountsAndDeclared(t *testing.T) {
	specs := requestFlowSpecs()
	declared := map[string]bool{}
	live := 0
	for _, s := range specs {
		if s.Declared {
			declared[s.Name] = true
		} else {
			live++
		}
	}
	if live != 17 || len(declared) != 3 || !declared["request_classify"] || !declared["request_generatePlan"] || !declared["request_startPhase"] {
		t.Fatalf("live=%d declared=%v, want 17 live and the 3 declared tools", live, declared)
	}
}

func TestRequestFlowPack_ListedWhenPacks1And2(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = map[int]bool{1: true, 2: true}
	cat, err := NewCatalog(AllSpecs(), cfg, &mcpservertest.FakeGate{}, productionRegistry().Channels())
	if err != nil {
		t.Fatal(err)
	}
	ts, err := cat.ListTools(context.Background(), alice)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	readOnly := map[string]bool{}
	for _, tl := range ts {
		if isRequestFlowNamespace(namespaceOf(channelOfTool(tl.Name))) {
			got = append(got, tl.Name)
			readOnly[tl.Name] = tl.Annotations.ReadOnlyHint
		}
	}
	sort.Strings(got)
	if len(got) != 17 {
		t.Fatalf("listed %d request-flow tools: %v", len(got), got)
	}
	for _, reads := range []string{"request_list", "request_get", "request_typeHistory", "solution_list", "approval_get", "approval_list",
		"approval_listPending", "backlog_requests", "backlog_tasks", "backlog_execute", "request_flowStatus"} {
		if !readOnly[reads] {
			t.Errorf("%s must be listed read-only", reads)
		}
	}
	for _, writes := range []string{"request_create", "request_changeType", "request_returnToBacklog", "request_reopen", "request_spawnChild", "solution_generate"} {
		if _, ok := readOnly[writes]; !ok || readOnly[writes] {
			t.Errorf("%s must be listed and not read-only", writes)
		}
	}
	for _, hidden := range []string{"request_classify", "request_generatePlan", "request_startPhase"} {
		if _, ok := readOnly[hidden]; ok {
			t.Errorf("declared tool %s must not be listed", hidden)
		}
	}
}

func channelOfTool(name string) string {
	for _, s := range AllSpecs() {
		if s.Name == name {
			return s.Channel
		}
	}
	return name
}

func TestRequestFlowPack_DefaultPackListsOnlyReads(t *testing.T) {
	cat, err := NewCatalog(AllSpecs(), DefaultConfig(), &mcpservertest.FakeGate{}, productionRegistry().Channels())
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := cat.ListTools(context.Background(), alice)
	n := 0
	for _, tl := range ts {
		if isRequestFlowNamespace(namespaceOf(channelOfTool(tl.Name))) {
			n++
			if !tl.Annotations.ReadOnlyHint {
				t.Errorf("%s listed under the default pack set but is not read-only", tl.Name)
			}
		}
	}
	if n != 11 {
		t.Errorf("default config lists %d request-flow tools, want the 11 reads", n)
	}
}

func TestRequestFlowPack_HumanGatesAreNotTools(t *testing.T) {
	ex := newRequestFlowExec(t, &fakeRequestService{}, DefaultConfig())
	for _, name := range []string{"approval_approve", "approval_reject", "approval_cancel", "solution_choose", "request_confirmType", "request_cancel", "request_flowSet", "request_subscribe"} {
		_, err := ex.CallTool(context.Background(), alice, name, json.RawMessage(`{}`))
		if err != mcpserver.ErrUnknownTool {
			t.Errorf("%s: got %v, want unknown tool", name, err)
		}
	}
	for _, name := range []string{"request_classify", "request_generatePlan", "request_startPhase"} {
		if _, err := ex.CallTool(context.Background(), alice, name, json.RawMessage(`{"id":"r"}`)); err != mcpserver.ErrUnknownTool {
			t.Errorf("declared %s must not run, got %v", name, err)
		}
	}
}

func TestRequestFlowPack_ScopeAndRisk(t *testing.T) {
	want := map[string]string{"request_get": "orca:read", "request_create": "orca:write", "solution_generate": "orca:write", "request_startPhase": "orca:exec"}
	for _, s := range requestFlowSpecs() {
		if scope, ok := want[s.Name]; ok && s.RequiredScope() != scope {
			t.Errorf("%s scope = %s, want %s", s.Name, s.RequiredScope(), scope)
		}
		if s.Risk == "read" && !s.Annotations.ReadOnly {
			t.Errorf("%s: read tool without readOnlyHint", s.Name)
		}
	}
	ex := newRequestFlowExec(t, &fakeRequestService{}, DefaultConfig())
	_, err := ex.CallTool(context.Background(), readerOnly, "request_create", json.RawMessage(`{"project_id":"p","title":"t"}`))
	if err == nil || errText(err)[:len("MCP_SCOPE_NOT_ALLOWED")] != "MCP_SCOPE_NOT_ALLOWED" {
		t.Errorf("a read-only token must not create Requests, got %v", err)
	}
}

// Identity and source are never tool input: they come from the principal and the MCP session.
func TestRequestFlowPack_SchemaHasNoIdentityFields(t *testing.T) {
	forbidden := []string{"tenant_id", "user_id", "reporter_id", "source_provider", "source_site", "tenantId", "userId", "reporterId", "sourceProvider"}
	for _, s := range requestFlowSpecs() {
		if err := s.prepare(); err != nil {
			t.Fatal(err)
		}
		for _, f := range forbidden {
			// request_list may filter by source_provider (a read filter, not a write claim).
			if f == "source_provider" && s.Name == "request_list" {
				continue
			}
			if _, bad := s.InputSchema().Properties[f]; bad {
				t.Errorf("%s exposes %s in its input schema", s.Name, f)
			}
		}
		for _, fl := range s.Fields {
			if fl.Wire == "tenantId" || fl.Wire == "userId" || fl.Wire == "reporterId" {
				t.Errorf("%s maps %s onto a wire identity key", s.Name, fl.Name)
			}
		}
	}
}

func TestRequestFlowPack_EveryChannelIsToolOrExcluded(t *testing.T) {
	ex, err := LoadExclusions()
	if err != nil {
		t.Fatal(err)
	}
	covers := specCovers(AllSpecs())
	for _, ch := range productionRegistry().Channels() {
		if !isRequestFlowNamespace(namespaceOf(ch.Name)) {
			continue
		}
		_, spec := covers[ch.Name]
		_, excluded := ex.Find(ch.Name)
		if spec == excluded {
			t.Errorf("%s: spec=%v excluded=%v, want exactly one", ch.Name, spec, excluded)
		}
	}
}
