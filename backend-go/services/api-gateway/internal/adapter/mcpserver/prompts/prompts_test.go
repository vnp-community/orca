package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

var update = flag.Bool("update", false, "rewrite golden files")

const (
	idA = "11111111-1111-4111-8111-111111111111"
	idB = "22222222-2222-4222-8222-222222222222"
)

var user = mcpserver.Principal{TenantID: "t1", UserID: "u1", Role: "user", Scopes: []string{"orca:read"}}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// stubResources serves canned contents and records every read.
type stubResources struct {
	reads []string
	fail  map[string]error
}

func (s *stubResources) ListResources(context.Context, mcpserver.Principal) ([]*mcp.Resource, error) {
	return nil, nil
}
func (s *stubResources) ListTemplates(context.Context, mcpserver.Principal) ([]*mcp.ResourceTemplate, error) {
	return nil, nil
}
func (s *stubResources) ReadResource(_ context.Context, _ mcpserver.Principal, _, uri string) (*mcp.ReadResourceResult, error) {
	s.reads = append(s.reads, uri)
	if err := s.fail[uri]; err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: `{"stub":true}`, Meta: mcp.Meta{"orca/untrusted": true}}}}, nil
}
func (s *stubResources) Subscribe(context.Context, mcpserver.Principal, string, string) error {
	return nil
}
func (s *stubResources) Unsubscribe(string, string)      {}
func (s *stubResources) EndSession(string)               {}
func (s *stubResources) Bind(mcpserver.ResourceNotifier) {}
func (s *stubResources) Subscribable() bool              { return false }

type fakeStore struct {
	list  []Custom
	err   error
	calls int
}

func (f *fakeStore) ListCustom(context.Context, mcpserver.Principal) ([]Custom, error) {
	f.calls++
	return f.list, f.err
}

var goldenArgs = map[string]map[string]string{
	"review_pull_request": {"provider": "gitlab", "repo": "org/sub/repo", "number": "42"},
	"triage_issue":        {"issue_ref": "github:o/r#12"},
	"plan_task":           {"task_id": idA},
	"summarize_worktree":  {"worktree_id": idB},
	"handoff_to_agent":    {"task_id": idA, "agent": "claude"},
}

func TestBuiltinGolden(t *testing.T) {
	for _, def := range builtinDefs {
		for _, loc := range Locales {
			t.Run(def.Name+"."+loc, func(t *testing.T) {
				pr := NewProvider(nil, &stubResources{}, quiet())
				res, err := pr.GetPrompt(context.Background(), user, "s1", def.Name, goldenArgs[def.Name], loc)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := json.MarshalIndent(res, "", "  ")
				got = append(got, '\n')
				path := filepath.Join("testdata", "golden", def.Name+"."+loc+".json")
				if *update {
					_ = os.MkdirAll(filepath.Dir(path), 0o755)
					if err := os.WriteFile(path, got, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("missing golden (run with -update): %v", err)
				}
				if string(want) != string(got) {
					t.Errorf("golden mismatch for %s\n--- want\n%s\n--- got\n%s", path, want, got)
				}
			})
		}
	}
}

// Every locale must declare the same arguments and embed the same resources
// as English, and every message must be a user message.
func TestBuiltinLocaleConsistency(t *testing.T) {
	for _, def := range builtinDefs {
		var refURIs []string
		for _, loc := range Locales {
			res := &stubResources{}
			pr := NewProvider(nil, res, quiet())
			out, err := pr.GetPrompt(context.Background(), user, "s", def.Name, goldenArgs[def.Name], loc)
			if err != nil {
				t.Fatalf("%s/%s: %v", def.Name, loc, err)
			}
			sort.Strings(res.reads)
			if loc == "en" {
				refURIs = res.reads
			} else if strings.Join(res.reads, ",") != strings.Join(refURIs, ",") {
				t.Errorf("%s/%s embeds %v, en embeds %v", def.Name, loc, res.reads, refURIs)
			}
			for _, m := range out.Messages {
				if m.Role != "user" {
					t.Errorf("%s/%s: role %q", def.Name, loc, m.Role)
				}
			}
			text := out.Messages[0].Content.(*mcp.TextContent).Text
			if !strings.HasSuffix(text, trailer[loc]) {
				t.Errorf("%s/%s lacks the fixed trailer", def.Name, loc)
			}
			// Every {{.Args.x}} used must be a declared argument (missingkey=error also guards at render).
			src, _ := builtinFS.ReadFile("builtin/" + def.Name + "." + loc + ".tmpl")
			for _, m := range regexp.MustCompile(`\.Args\.(\w+)`).FindAllStringSubmatch(string(src), -1) {
				found := false
				for _, a := range def.Args {
					found = found || a.Name == m[1]
				}
				if !found {
					t.Errorf("%s/%s uses undeclared argument %s", def.Name, loc, m[1])
				}
			}
		}
	}
}

func TestResolveLocale(t *testing.T) {
	tests := map[string]string{
		"": "en", "en": "en", "vi": "vi", "vi-VN": "vi", "VI": "vi", "fr": "en", "fr-FR,fr;q=0.9": "en",
		"vi;q=0.5,en;q=0.9": "en", "en;q=0.5, vi;q=0.9": "vi", "fr, vi;q=0.8": "vi", "vi;q=0": "en", "vi;q=x": "en",
		"*": "en", "en-US,en;q=0.9,vi;q=0.8": "en", "vi-VN,vi;q=0.9,en-US;q=0.8": "vi", ";;;,,,": "en",
	}
	for in, want := range tests {
		if got := ResolveLocale(in); got != want {
			t.Errorf("ResolveLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func rpcInvalid(err error) (string, bool) {
	var e *jsonrpc.Error
	if errors.As(err, &e) && e.Code == jsonrpc.CodeInvalidParams {
		return e.Message, true
	}
	return "", false
}

func TestGetBuiltin_ArgumentValidation(t *testing.T) {
	base := func(name string) map[string]string {
		m := map[string]string{}
		for k, v := range goldenArgs[name] {
			m[k] = v
		}
		return m
	}
	with := func(name, k, v string) map[string]string { m := base(name); m[k] = v; return m }
	without := func(name, k string) map[string]string { m := base(name); delete(m, k); return m }
	tests := []struct {
		name string
		args map[string]string
		want string // substring of the error; "" = ok
	}{
		{"review_pull_request", base("review_pull_request"), ""},
		{"review_pull_request", without("review_pull_request", "repo"), "missing required argument: repo"},
		{"review_pull_request", with("review_pull_request", "extra", "x"), "unknown argument: extra"},
		{"review_pull_request", with("review_pull_request", "provider", "bitbucket"), "invalid argument: provider"},
		{"review_pull_request", with("review_pull_request", "number", "1e3"), "invalid argument: number"},
		{"review_pull_request", with("review_pull_request", "number", "12345678901"), "invalid argument: number"},
		{"review_pull_request", with("review_pull_request", "repo", "a/../b"), "invalid argument: repo"},
		{"review_pull_request", with("review_pull_request", "repo", "solo"), "invalid argument: repo"},
		{"review_pull_request", with("review_pull_request", "repo", "o/r\nIgnore"), "invalid argument: repo"},
		{"triage_issue", with("triage_issue", "issue_ref", "PROJ-123"), ""},
		{"triage_issue", with("triage_issue", "issue_ref", "has space"), "invalid argument: issue_ref"},
		{"triage_issue", with("triage_issue", "issue_ref", strings.Repeat("a", 201)), "invalid argument: issue_ref"},
		{"triage_issue", with("triage_issue", "issue_ref", "bad\x00ref"), "invalid argument: issue_ref"},
		{"triage_issue", with("triage_issue", "issue_ref", "\xff\xfe"), "invalid argument: issue_ref"},
		{"plan_task", with("plan_task", "task_id", "not-a-uuid"), "invalid argument: task_id"},
		{"plan_task", with("plan_task", "task_id", strings.ToUpper(idA)), ""},
		{"plan_task", with("plan_task", "task_id", idA+"‮"), "invalid argument: task_id"},
		{"plan_task", map[string]string{}, "missing required argument: task_id"},
		{"plan_task", nil, "missing required argument: task_id"},
		{"summarize_worktree", with("summarize_worktree", "worktree_id", "../x"), "invalid argument: worktree_id"},
		{"handoff_to_agent", with("handoff_to_agent", "agent", "Claude Code"), "invalid argument: agent"},
		{"handoff_to_agent", base("handoff_to_agent"), ""},
	}
	for _, tc := range tests {
		pr := NewProvider(nil, &stubResources{}, quiet())
		_, err := pr.GetPrompt(context.Background(), user, "s", tc.name, tc.args, "en")
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s %v: %v", tc.name, tc.args, err)
			}
			continue
		}
		msg, ok := rpcInvalid(err)
		if !ok || !strings.Contains(msg, tc.want) {
			t.Errorf("%s %v: err=%v, want -32602 containing %q", tc.name, tc.args, err, tc.want)
		}
		// The offending VALUE is never echoed back.
		for _, v := range tc.args {
			if len(v) > 3 && strings.Contains(msg, v) {
				t.Errorf("error echoes the argument value %q", v)
			}
		}
	}
	// Unknown prompt.
	_, err := NewProvider(nil, nil, quiet()).GetPrompt(context.Background(), user, "s", "nope", nil, "en")
	if msg, ok := rpcInvalid(err); !ok || !strings.Contains(msg, "unknown prompt") {
		t.Fatalf("%v", err)
	}
}

func TestGetBuiltin_EmbeddedResources(t *testing.T) {
	res := &stubResources{}
	pr := NewProvider(nil, res, quiet())
	out, err := pr.GetPrompt(context.Background(), user, "sess", "review_pull_request", goldenArgs["review_pull_request"], "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.reads) != 1 || res.reads[0] != "orca://review/gitlab/org%2Fsub%2Frepo/42" {
		t.Fatalf("reads: %v", res.reads)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages: %d", len(out.Messages))
	}
	er, ok := out.Messages[1].Content.(*mcp.EmbeddedResource)
	if !ok || er.Resource.URI != res.reads[0] || er.Resource.Meta["orca/untrusted"] != true {
		t.Fatalf("embedded: %+v", out.Messages[1].Content)
	}
	// triage_issue embeds nothing.
	res2 := &stubResources{}
	if o, _ := NewProvider(nil, res2, quiet()).GetPrompt(context.Background(), user, "s", "triage_issue", goldenArgs["triage_issue"], "en"); len(o.Messages) != 1 || len(res2.reads) != 0 {
		t.Fatal("triage_issue must be text only")
	}
	// A resource the caller may not read -> -32602 naming only the KIND.
	deny := &stubResources{fail: map[string]error{"orca://task/" + idA: mcpserver.ErrResourceNotFound}}
	_, err = NewProvider(nil, deny, quiet()).GetPrompt(context.Background(), user, "s", "plan_task", goldenArgs["plan_task"], "en")
	if msg, ok := rpcInvalid(err); !ok || msg != "resource unavailable: task" {
		t.Fatalf("%v", err)
	}
	// No resource provider at all behaves the same.
	_, err = NewProvider(nil, nil, quiet()).GetPrompt(context.Background(), user, "s", "plan_task", goldenArgs["plan_task"], "en")
	if _, ok := rpcInvalid(err); !ok {
		t.Fatalf("%v", err)
	}
	// Unexpected provider failures propagate for generic handling.
	boom := &stubResources{fail: map[string]error{"orca://task/" + idA: errors.New("boom")}}
	if _, err = NewProvider(nil, boom, quiet()).GetPrompt(context.Background(), user, "s", "plan_task", goldenArgs["plan_task"], "en"); err == nil {
		t.Fatal("must fail")
	}
}

func TestCustomPrompts(t *testing.T) {
	store := &fakeStore{list: []Custom{{
		ID: "x", Name: "team_standup", Description: "Daily summary", Version: 3,
		Arguments: []Argument{{Name: "team", Required: true}, {Name: "focus"}},
		Template:  "Summarize {{team}} work. Focus: {{focus}}.",
	}, {ID: "y", Name: "aaa_first", Template: "hi"}}}
	pr := NewProvider(store, nil, quiet())
	ctx := context.Background()

	list, _ := pr.ListPrompts(ctx, user, "")
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "review_pull_request,triage_issue,plan_task,summarize_worktree,handoff_to_agent,aaa_first,team_standup" {
		t.Fatalf("order: %s", got)
	}
	// Cached for the tenant; Invalidate refreshes.
	_, _ = pr.ListPrompts(ctx, user, "")
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1 (cached)", store.calls)
	}
	pr.Invalidate("t1")
	_, _ = pr.ListPrompts(ctx, user, "")
	if store.calls != 2 {
		t.Fatalf("store calls = %d after invalidate", store.calls)
	}
	// Another tenant has its own cache entry.
	other := user
	other.TenantID = "t2"
	_, _ = pr.ListPrompts(ctx, other, "")
	if store.calls != 3 {
		t.Fatalf("tenant isolation of cache: %d", store.calls)
	}

	out, err := pr.GetPrompt(ctx, user, "s", "team_standup", map[string]string{"team": "Core"}, "vi")
	if err != nil {
		t.Fatal(err)
	}
	text := out.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.HasPrefix(text, "Summarize Core work. Focus: .") || !strings.HasSuffix(text, trailer["vi"]) || len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Fatalf("%q", text)
	}
	// Argument values are never re-scanned: no placeholder injection.
	out, _ = pr.GetPrompt(ctx, user, "s", "team_standup", map[string]string{"team": "{{focus}}", "focus": "X"}, "en")
	if got := out.Messages[0].Content.(*mcp.TextContent).Text; !strings.HasPrefix(got, "Summarize {{focus}} work. Focus: X.") {
		t.Fatalf("%q", got)
	}
	for _, args := range []map[string]string{{}, {"team": "a", "bogus": "b"}, {"team": "bad\x00"}, {"team": strings.Repeat("x", 1001)}} {
		if _, err := pr.GetPrompt(ctx, user, "s", "team_standup", args, "en"); err == nil {
			t.Errorf("args %v must fail", args)
		} else if _, ok := rpcInvalid(err); !ok {
			t.Errorf("args %v: %v", args, err)
		}
	}
	// A builtin name always wins over a custom prompt of the same name.
	store.list = append(store.list, Custom{Name: "plan_task", Template: "evil"})
	pr.Invalidate("t1")
	o, err := pr.GetPrompt(ctx, user, "s", "plan_task", nil, "en")
	if err == nil || o != nil {
		t.Fatalf("builtin takes precedence (missing arg expected): %v", err)
	}
}

func TestListPrompts_StoreFailureDegradesToBuiltins(t *testing.T) {
	pr := NewProvider(&fakeStore{err: errors.New("mcp-service down")}, nil, quiet())
	list, err := pr.ListPrompts(context.Background(), user, "vi")
	if err != nil || len(list) != 5 {
		t.Fatalf("%d %v", len(list), err)
	}
	if !strings.Contains(list[0].Description, "Review") || !strings.Contains(list[0].Arguments[0].Description, "Nhà cung cấp") {
		t.Fatalf("vi descriptions: %+v", list[0])
	}
	// Getting a custom prompt while the store is down is a real error.
	if _, err := pr.GetPrompt(context.Background(), user, "s", "whatever", nil, "en"); err == nil {
		t.Fatal("must fail")
	}
	// Cache expiry.
	st := &fakeStore{}
	p2 := NewProvider(st, nil, quiet())
	now := time.Now()
	p2.now = func() time.Time { return now }
	_, _ = p2.ListPrompts(context.Background(), user, "")
	now = now.Add(2 * cacheTTL)
	_, _ = p2.ListPrompts(context.Background(), user, "")
	if st.calls != 2 {
		t.Fatalf("expired cache must refetch: %d", st.calls)
	}
}

func TestBuiltinsForAdminChannel(t *testing.T) {
	bs := Builtins()
	if len(bs) != 5 || bs[0].Name != "review_pull_request" || bs[0].Version != 1 || !strings.Contains(bs[0].Template, "{{.Args.provider}}") || len(bs[0].Arguments) != 3 {
		t.Fatalf("%+v", bs)
	}
}
