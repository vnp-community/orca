package tools

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

func names(ts []string) string { sort.Strings(ts); return "," + strings.Join(ts, ",") + "," }

func listNames(t *testing.T, cat *Catalog, p mcpserver.Principal) []string {
	t.Helper()
	ts, err := cat.ListTools(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(ts))
	for _, x := range ts {
		out = append(out, x.Name)
	}
	return out
}

func TestDefaultConfigListsOnlyPack1Read(t *testing.T) {
	cat, err := NewCatalog(AllSpecs(), DefaultConfig(), nil, productionRegistry().Channels())
	if err != nil {
		t.Fatal(err)
	}
	got := listNames(t, cat, alice)
	if len(got) == 0 {
		t.Fatal("no tools")
	}
	for _, s := range cat.Specs() {
		if s.Pack != 1 {
			t.Errorf("pack %d tool %s leaked into the default catalog", s.Pack, s.Name)
		}
	}
	for _, n := range got {
		// terminal_list is a pack 1 read; every exec terminal/agent tool is pack 3.
		if (strings.HasPrefix(n, "terminal_") && n != "terminal_list") || strings.HasPrefix(n, "agent_") {
			t.Errorf("BE-009 exec tool %s must not be in the default catalog", n)
		}
	}
}

func TestFailClosedGateHidesNonReadEvenWhenPacksEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = allPacks
	cat, _ := NewCatalog(AllSpecs(), cfg, nil, nil) // nil gate -> FailClosedGate
	ts, _ := cat.ListTools(context.Background(), alice)
	for _, tool := range ts {
		s, _ := cat.Lookup(tool.Name)
		if s.Risk != "read" {
			t.Errorf("%s (%s) visible under the fail-closed default", s.Name, s.Risk)
		}
	}
	ex := NewExecutor(cat, fakeRegistry(nil, nil), nil, nil, cfg, quiet)
	if _, err := ex.CallTool(context.Background(), alice, "task_create", json.RawMessage(`{"title":"x"}`)); err == nil ||
		!strings.HasPrefix(errText(err), "MCP_POLICY_DENIED") {
		t.Errorf("write tool must be denied without a gate: %v", err)
	}
	if _, err := ex.CallTool(context.Background(), alice, "project_list", json.RawMessage(`{}`)); err != nil {
		t.Errorf("read tool must work without a gate: %v", err)
	}
}

func TestListFiltersByScopeAndPolicyView(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = map[int]bool{1: true, 2: true}
	gate := mcpservertest.FakeViewGate{FakeGate: &mcpservertest.FakeGate{Effective: func(m mcpserver.ToolMeta) mcpserver.EffectiveDecision {
		switch m.Name {
		case "task_create":
			return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeRequireApproval, Source: "tenant_policy"}
		case "git_commit":
			return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeDeny, Source: "tenant_policy"}
		}
		return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow, Source: "default"}
	}}}
	cat, err := NewCatalog(AllSpecs(), cfg, gate, nil)
	if err != nil {
		t.Fatal(err)
	}
	all := names(listNames(t, cat, alice))
	if !strings.Contains(all, ",task_create,") || strings.Contains(all, ",git_commit,") || !strings.Contains(all, ",project_list,") {
		t.Error("require_approval must stay listed and deny must be hidden")
	}
	read := names(listNames(t, cat, readerOnly))
	if strings.Contains(read, ",task_create,") || strings.Contains(read, ",git_stage,") || !strings.Contains(read, ",project_list,") {
		t.Error("read-only token must only see read tools")
	}
	cat.InvalidateTenant("t1") // must not panic; clears cached decisions
}

func TestAdminToolViews(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Packs = allPacks
	gate := mcpservertest.FakeViewGate{FakeGate: &mcpservertest.FakeGate{Effective: func(m mcpserver.ToolMeta) mcpserver.EffectiveDecision {
		if m.Risk == "destructive" {
			return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeDeny, Source: "kill_switch"}
		}
		return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow, Source: "default"}
	}}}
	cat, err := NewCatalog(AllSpecs(), cfg, gate, productionRegistry().Channels())
	if err != nil {
		t.Fatal(err)
	}
	views, _ := cat.AdminToolViews(context.Background(), "t1")
	byName := map[string]int{}
	var hard int
	for i, v := range views {
		byName[v.Name] = i
		if v.HardDenied {
			hard++
			if v.Effective != "deny" || v.EffectiveSource != "hard_deny" || v.Pack != 4 || v.Risk != "admin" {
				t.Errorf("hard-denied view %+v", v)
			}
		}
	}
	if hard == 0 {
		t.Error("expected hard-denied entries from excluded_channels.yaml")
	}
	if _, ok := byName["credentials_list"]; !ok {
		t.Error("credentials.* should be listed as hard denied")
	}
	rm := views[byName["worktree_rm"]]
	if rm.Effective != "deny" || rm.EffectiveSource != "kill_switch" || rm.RequiredScope != "orca:admin" || !rm.Annotations.Destructive || rm.Pack != 4 {
		t.Errorf("worktree_rm view %+v", rm)
	}
	st := views[byName["git_status"]]
	if st.Risk != "read" || !st.Annotations.ReadOnly || st.Namespace != "git" || st.Channel != "git.status" || st.Effective != "allow" {
		t.Errorf("git_status view %+v", st)
	}
	// declared (not runnable) tools are still shown to admins
	if _, ok := byName["task_aiApply"]; !ok {
		t.Error("declared tool missing from admin view")
	}
}

type goldenTool struct {
	Name     string   `json:"name"`
	Channel  string   `json:"channel"`
	Pack     int      `json:"pack"`
	Risk     string   `json:"risk"`
	Scope    string   `json:"scope"`
	Declared bool     `json:"declared,omitempty"`
	Required []string `json:"required,omitempty"`
	Props    []string `json:"props,omitempty"`
}

func TestToolsListGolden(t *testing.T) {
	var out []goldenTool
	for _, s := range AllSpecs() {
		if err := s.prepare(); err != nil {
			t.Fatal(err)
		}
		g := goldenTool{Name: s.Name, Channel: s.Channel, Pack: s.Pack, Risk: s.Risk, Scope: s.RequiredScope(), Declared: s.Declared}
		g.Required = append(g.Required, s.input.Required...)
		for k := range s.input.Properties {
			g.Props = append(g.Props, k)
		}
		sort.Strings(g.Required)
		sort.Strings(g.Props)
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	b, _ := json.MarshalIndent(out, "", " ")
	b = append(b, '\n')
	const path = "testdata/tools_list.golden.json"
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	// Why semantic: the repo's pre-commit formatter rewrites committed JSON, so a byte
	// comparison would fail on a clean checkout even when nothing changed.
	if !sameJSON(t, want, b) {
		t.Error("tools list changed; review the diff and run `go test -run TestToolsListGolden -update`")
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("golden is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("generated list is not valid JSON: %v", err)
	}
	return reflect.DeepEqual(av, bv)
}

func TestInventoryCountsByPack(t *testing.T) {
	perPack := map[int][2]int{} // [implemented, declared]
	for _, s := range AllSpecs() {
		c := perPack[s.Pack]
		if s.Declared {
			c[1]++
		} else {
			c[0]++
		}
		perPack[s.Pack] = c
	}
	t.Logf("MCP_TOOL_INVENTORY pack1=%v pack2=%v pack3=%v pack4=%v (implemented,declared)", perPack[1], perPack[2], perPack[3], perPack[4])
}
