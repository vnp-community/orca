package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

func regoDecision(t *testing.T, ev *policy.Evaluator, s *ToolSpec, role string, scopes []string) map[string]any {
	t.Helper()
	m := s.Meta()
	in := map[string]any{
		"user":       map[string]any{"id": "u1", "role": role},
		"client":     map[string]any{"id": "c1", "status": "allowed"},
		"token":      map[string]any{"scopes": scopes},
		"settings":   map[string]any{"enabled": true, "max_depth": 1}, // default tenant settings
		"killswitch": map[string]any{"active": false},
		"session":    map[string]any{"depth": 0, "untrusted_read": false},
		"tool": map[string]any{"name": m.Name, "channel": m.Channel, "namespace": m.Namespace, "risk": m.Risk,
			"required_scope": m.RequiredScope, "open_world": m.OpenWorld, "spawns_process": false},
		"tenant_policies": []any{},
	}
	v, err := ev.Value(context.Background(), "data.orca.authz.mcp.decision", in)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}

func newRegoEvaluator() *policy.Evaluator {
	_, file, _, _ := runtime.Caller(0)
	return policy.NewEvaluator(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "policy", "orca-authz"))
}

// Every pack 4 tool is destructive/admin, so the real Rego bundle never lets
// it run unattended under default tenant settings.
func TestPack4NeverAllowedByDefaultRego(t *testing.T) {
	ev := newRegoEvaluator()
	n := 0
	for _, s := range AllSpecs() {
		if s.Pack != 4 {
			continue
		}
		n++
		if s.Risk != "destructive" && s.Risk != "admin" {
			t.Errorf("%s: pack 4 tool with risk %s", s.Name, s.Risk)
		}
		if want := map[string]string{"destructive": "orca:admin", "admin": "orca:admin"}[s.Risk]; s.RequiredScope() != want {
			t.Errorf("%s: scope %s", s.Name, s.RequiredScope())
		}
		if s.Risk == "destructive" && !s.Annotations.Destructive {
			t.Errorf("%s: missing destructiveHint", s.Name)
		}
		all := []string{"orca:read", "orca:write", "orca:exec", "orca:admin"}
		for _, role := range []string{"user", "admin"} {
			d := regoDecision(t, ev, s, role, all)
			got := d["decision"]
			if got == "allow" || (s.Risk == "destructive" && got != "require_approval") {
				t.Errorf("%s as %s: decision %v (%v)", s.Name, role, got, d["reasons"])
			}
		}
	}
	if n == 0 {
		t.Fatal("no pack 4 tools")
	}
}

// The Rego hard-deny list wins: none of its channels may be a tool, and a
// disguised destructive tool on such a channel is still denied.
func TestHardDeniedChannelsAreNotTools(t *testing.T) {
	ev := newRegoEvaluator()
	v, err := ev.Value(context.Background(), "data.orca.authz.mcp.hard_deny_channels", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	prefixes, _ := ev.Value(context.Background(), "data.orca.authz.mcp.hard_deny_prefixes", map[string]any{})
	var denied []string
	for _, x := range v.([]any) {
		denied = append(denied, x.(string))
	}
	ex, _ := LoadExclusions()
	for _, ch := range denied {
		if _, ok := ex.Find(ch); !ok {
			t.Errorf("hard-denied channel %s is not in excluded_channels.yaml", ch)
		}
	}
	for _, s := range AllSpecs() {
		for _, ch := range append([]string{s.Channel}, s.UsesChannels...) {
			for _, d := range denied {
				if ch == d {
					t.Errorf("tool %s exposes hard-denied channel %s", s.Name, ch)
				}
			}
			for _, p := range prefixes.([]any) {
				if len(ch) >= len(p.(string)) && ch[:len(p.(string))] == p.(string) {
					t.Errorf("tool %s exposes hard-denied namespace %s", s.Name, ch)
				}
			}
		}
	}
	probe := newSpec(4, "destructive", "team.create", "x", nil)
	if d := regoDecision(t, ev, probe, "admin", []string{"orca:admin"}); d["decision"] != "deny" || d["source"] != "hard_deny" {
		t.Errorf("hard-deny must win: %v", d)
	}
}

func TestPack4HiddenUnlessEnabled(t *testing.T) {
	cat, err := NewCatalog(AllSpecs(), DefaultConfig(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cat.Specs() {
		if s.Pack == 4 {
			t.Errorf("%s leaked with the default packs", s.Name)
		}
	}
	p, err := ParsePacks("1,4")
	if err != nil || !p[4] || !p[1] || p[2] {
		t.Fatalf("ParsePacks: %v %v", p, err)
	}
	if d := DefaultConfig().Packs; len(d) != 1 || !d[1] {
		t.Errorf("default must stay pack 1: %v", d)
	}
}

func TestPack4ArgsMapping(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"annotation_delete":                       {`{"id":"a1","confirmed":true}`, `{"confirmed":true,"id":"a1"}`},
		"projectGroup_delete":                     {`{"group_id":"g1"}`, `{"groupId":"g1"}`},
		"github_project_deleteIssueCommentBySlug": {`{"item_slug":"o/r#1","comment_id":"9"}`, `{"commentId":"9","itemSlug":"o/r#1"}`},
		"worktree_rm":                             {`{"worktree_id":"w1","force":true}`, `{"force":true,"worktree":"id:w1"}`},
	}
	byName := map[string]*ToolSpec{}
	for _, s := range AllSpecs() {
		byName[s.Name] = s
	}
	for name, c := range cases {
		s := byName[name]
		if s == nil {
			t.Errorf("%s missing", name)
			continue
		}
		args, err := s.buildArgs(json.RawMessage(c.in), wscompat.Identity{TenantID: "t1", UserID: "u1"}, DefaultConfig())
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		var a, b any
		_ = json.Unmarshal(args[0], &a)
		_ = json.Unmarshal([]byte(c.want), &b)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: got %s want %s", name, args[0], c.want)
		}
	}
}
