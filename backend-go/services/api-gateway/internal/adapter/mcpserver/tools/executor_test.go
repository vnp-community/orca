package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type recorded struct {
	channel string
	id      wscompat.Identity
	args    []json.RawMessage
}

// fakeRegistry builds a real *wscompat.Registry with a few canned channels.
func fakeRegistry(calls *[]recorded, hits *atomic.Int32) *wscompat.Registry {
	r := wscompat.NewRegistry()
	rec := func(channel string, out any, err error) {
		r.Register(channel, func(_ context.Context, id wscompat.Identity, args []json.RawMessage) (any, error) {
			if calls != nil {
				*calls = append(*calls, recorded{channel, id, args})
			}
			if hits != nil {
				hits.Add(1)
			}
			return out, err
		})
	}
	rec("project.list", []map[string]any{{"id": "p1", "default_branch": "main"}}, nil)
	rec("git.status", map[string]any{"files": []string{"a.go"}, "remote_url": "https://u:ghp_abcdefghijklmnopqrstuvwxyz0123456789@github.com/o/r.git"}, nil)
	rec("task.create", map[string]any{"id": "t1"}, nil)
	rec("worktree.rm", map[string]any{"ok": true}, nil)
	rec("github.issues", []any{map[string]any{"number": 1}}, nil)
	rec("project.get", nil, status.Error(codes.NotFound, "project abc not found in db shard-3"))
	rec("repo.list", nil, status.Error(codes.Internal, "pq: password authentication failed"))
	rec("worktree.list", nil, errors.New("WORKTREE_X: failed for /home/secret/path"))
	r.RegisterStreamChannel("task.get", func(_ context.Context, _ wscompat.Identity, _ []json.RawMessage) (any, <-chan wscompat.PushEvent, error) {
		ch := make(chan wscompat.PushEvent)
		return map[string]any{"ack": true}, ch, nil
	})
	return r
}

func newExec(t *testing.T, gate mcpserver.PolicyGate, packs map[int]bool, calls *[]recorded, hits *atomic.Int32) (*Executor, *Catalog) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Packs = packs
	reg := fakeRegistry(calls, hits)
	cat, err := NewCatalog(AllSpecs(), cfg, gate, reg.Channels())
	if err != nil {
		t.Fatal(err)
	}
	return NewExecutor(cat, reg, gate, nil, cfg, quiet), cat
}

var (
	allPacks   = map[int]bool{1: true, 2: true, 3: true, 4: true}
	allScopes  = []string{"orca:read", "orca:write", "orca:exec", "orca:admin"}
	alice      = mcpserver.Principal{TenantID: "t1", UserID: "alice", Role: "user", Scopes: allScopes}
	readerOnly = mcpserver.Principal{TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}}
)

func errText(err error) string { return mcpserver.MapToolError(err) }

func TestCallToolHappyPathIdentityRedactionAndNormalization(t *testing.T) {
	var calls []recorded
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, &calls, nil)
	res, err := ex.CallTool(context.Background(), alice, "git_status", json.RawMessage(`{"worktree_id":"w1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].channel != "git.status" || calls[0].id != (wscompat.Identity{TenantID: "t1", UserID: "alice", Role: "user"}) {
		t.Fatalf("dispatch %+v", calls)
	}
	if string(calls[0].args[0]) != `{"worktree":"id:w1"}` {
		t.Errorf("args %s", calls[0].args[0])
	}
	sc := res.StructuredContent.(map[string]any)
	if sc["remoteUrl"] != "https://***@github.com/o/r.git" {
		t.Errorf("remote url not redacted/camelized: %v", sc)
	}
	if txt, ok := res.Content[0].(*mcp.TextContent); !ok || strings.Contains(txt.Text, "ghp_") {
		t.Errorf("text content must be redacted JSON: %#v", res.Content[0])
	}
}

func TestScopeEnforcementTable(t *testing.T) {
	cases := []struct {
		tool   string
		scopes []string
		ok     bool
	}{
		{"project_list", []string{"orca:read"}, true},
		{"project_list", []string{"orca:write"}, false},
		{"task_create", []string{"orca:read"}, false},
		{"task_create", []string{"orca:write"}, true},
		{"worktree_rm", []string{"orca:read", "orca:write", "orca:exec"}, false},
		{"worktree_rm", []string{"orca:admin"}, true},
		{"project_list", nil, false},
	}
	for _, c := range cases {
		ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, nil)
		p := alice
		p.Scopes = c.scopes
		in := json.RawMessage(`{}`)
		switch c.tool {
		case "task_create":
			in = json.RawMessage(`{"title":"x"}`)
		case "worktree_rm":
			in = json.RawMessage(`{"worktree_id":"w"}`)
		}
		_, err := ex.CallTool(context.Background(), p, c.tool, in)
		if c.ok && err != nil {
			t.Errorf("%s %v: %v", c.tool, c.scopes, err)
		}
		if !c.ok && (err == nil || !strings.HasPrefix(errText(err), "MCP_SCOPE_NOT_ALLOWED")) {
			t.Errorf("%s %v: want scope error, got %v", c.tool, c.scopes, err)
		}
	}
}

func TestGateOutcomesFailClosed(t *testing.T) {
	deny := mcpserver.GateDecision{Outcome: mcpserver.OutcomeDeny, Message: "nope"}
	ask := mcpserver.GateDecision{Outcome: mcpserver.OutcomeRequireApproval, ApprovalID: "ap1"}
	cases := []struct {
		name     string
		gate     *mcpservertest.FakeGate
		wantCode string // "" = success
		reached  bool
	}{
		{"allow", &mcpservertest.FakeGate{}, "", true},
		{"deny", &mcpservertest.FakeGate{Default: deny}, "MCP_POLICY_DENIED", false},
		{"unknown outcome", &mcpservertest.FakeGate{Default: mcpserver.GateDecision{Outcome: "maybe"}}, "MCP_POLICY_DENIED", false},
		{"empty outcome is allow-by-fake only", &mcpservertest.FakeGate{Default: mcpserver.GateDecision{}}, "", true},
		{"decide error", &mcpservertest.FakeGate{DecideErr: errors.New("boom")}, "MCP_UNAVAILABLE", false},
		{"approval granted", &mcpservertest.FakeGate{Default: ask, Approved: true}, "", true},
		{"approval refused", &mcpservertest.FakeGate{Default: ask}, "MCP_APPROVAL_DENIED", false},
		{"approval error", &mcpservertest.FakeGate{Default: ask, Approved: true, ApproveErr: errors.New("x")}, "MCP_UNAVAILABLE", false},
		{"approval without id", &mcpservertest.FakeGate{Default: mcpserver.GateDecision{Outcome: mcpserver.OutcomeRequireApproval}, Approved: true}, "MCP_POLICY_DENIED", false},
	}
	for _, c := range cases {
		var hits atomic.Int32
		ex, _ := newExec(t, c.gate, allPacks, nil, &hits)
		_, err := ex.CallTool(context.Background(), alice, "task_create", json.RawMessage(`{"title":"x"}`))
		if c.wantCode == "" && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if c.wantCode != "" && (err == nil || !strings.HasPrefix(errText(err), c.wantCode)) {
			t.Errorf("%s: want %s got %v", c.name, c.wantCode, err)
		}
		if (hits.Load() == 1) != c.reached {
			t.Errorf("%s: handler reached=%d", c.name, hits.Load())
		}
		if c.reached && (len(c.gate.Completed) != 1 || c.gate.Completed[0].Result != "ok") {
			t.Errorf("%s: Complete not recorded: %+v", c.name, c.gate.Completed)
		}
		if !c.reached && len(c.gate.Completed) != 0 {
			t.Errorf("%s: Complete recorded for a call that never ran", c.name)
		}
	}
}

func TestCompleteReportsErrorAndMetaCarriesRisk(t *testing.T) {
	g := &mcpservertest.FakeGate{}
	ex, _ := newExec(t, g, allPacks, nil, nil)
	_, err := ex.CallTool(context.Background(), alice, "project_get", json.RawMessage(`{"project_id":"p"}`))
	if err == nil || errText(err) != "MCP_NOT_FOUND: not found or not permitted" {
		t.Fatalf("got %v / %q", err, errText(err))
	}
	if len(g.Completed) != 1 || g.Completed[0].Result != "error" || g.Completed[0].Meta.Risk != "read" ||
		g.Completed[0].Meta.RequiredScope != "orca:read" || g.Completed[0].Meta.Channel != "project.get" {
		t.Errorf("completion %+v", g.Completed)
	}
}

func TestErrorMappingNeverLeaksInternals(t *testing.T) {
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, nil)
	for tool, in := range map[string]string{"repo_list": `{"project_id":"p"}`, "worktree_list": `{"project_id":"p"}`} {
		_, err := ex.CallTool(context.Background(), alice, tool, json.RawMessage(in))
		got := errText(err)
		for _, leak := range []string{"password", "pq:", "/home/secret", "shard"} {
			if strings.Contains(got, leak) {
				t.Errorf("%s leaked %q: %s", tool, leak, got)
			}
		}
	}
	_, err := ex.CallTool(context.Background(), alice, "worktree_list", json.RawMessage(`{"project_id":"p"}`))
	if errText(err) != "WORKTREE_X: request failed" {
		t.Errorf("coded error: %q", errText(err))
	}
}

func TestUnknownHiddenAndDeclaredTools(t *testing.T) {
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, map[int]bool{1: true}, nil, nil)
	for _, name := range []string{"nope", "task_create", "worktree_rm", "task_aiApply"} {
		if _, err := ex.CallTool(context.Background(), alice, name, json.RawMessage(`{}`)); !errors.Is(err, mcpserver.ErrUnknownTool) {
			t.Errorf("%s: want ErrUnknownTool, got %v", name, err)
		}
	}
	ex, _ = newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, nil)
	if _, err := ex.CallTool(context.Background(), alice, "task_aiApply", json.RawMessage(`{"task_id":"t"}`)); !errors.Is(err, mcpserver.ErrUnknownTool) {
		t.Errorf("declared tool must not run: %v", err)
	}
}

func TestInvalidArgumentsDoNotReachGateOrHandler(t *testing.T) {
	g := &mcpservertest.FakeGate{}
	var hits atomic.Int32
	ex, _ := newExec(t, g, allPacks, nil, &hits)
	_, err := ex.CallTool(context.Background(), alice, "task_create", json.RawMessage(`{"title":"x","userId":"mallory"}`))
	if err == nil || !strings.HasPrefix(errText(err), "INVALID_ARGUMENTS") {
		t.Fatalf("got %v", err)
	}
	if len(g.Decided) != 0 || hits.Load() != 0 {
		t.Error("invalid input reached gate or handler")
	}
}

func TestStreamChannelUsesAckOnly(t *testing.T) {
	// task.get is registered as a streamChannel in this fake; a tool for it
	// must take the ack and not hang on events.
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, nil)
	res, err := ex.CallTool(context.Background(), alice, "task_get", json.RawMessage(`{"id":"t"}`))
	if err != nil || res.StructuredContent.(map[string]any)["ack"] != true {
		t.Fatalf("%v %v", res, err)
	}
}

func TestListResultsAreWrappedAndCached(t *testing.T) {
	var hits atomic.Int32
	ex, _ := newExec(t, &mcpservertest.FakeGate{}, allPacks, nil, &hits)
	now := time.Unix(1000, 0)
	ex.now = func() time.Time { return now }
	in := json.RawMessage(`{"repo":"r"}`)
	for i := 0; i < 2; i++ {
		res, err := ex.CallTool(context.Background(), alice, "github_issues", in)
		if err != nil {
			t.Fatal(err)
		}
		sc := res.StructuredContent.(map[string]any)
		if _, ok := sc["items"]; !ok || sc["untrusted"] != true {
			t.Errorf("envelope %v", sc)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("30s cache: handler hit %d times", hits.Load())
	}
	other := alice
	other.UserID = "bob"
	_, _ = ex.CallTool(context.Background(), other, "github_issues", in)
	if hits.Load() != 2 {
		t.Error("cache must be per user")
	}
	now = now.Add(31 * time.Second)
	_, _ = ex.CallTool(context.Background(), alice, "github_issues", in)
	if hits.Load() != 3 {
		t.Error("cache must expire after 30s")
	}
}

func TestSessionCloseCancelsCall(t *testing.T) {
	reg := wscompat.NewRegistry()
	started := make(chan struct{})
	reg.Register("project.list", func(ctx context.Context, _ wscompat.Identity, _ []json.RawMessage) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	cfg := DefaultConfig()
	cat, _ := NewCatalog(AllSpecs(), cfg, nil, reg.Channels())
	sess := NewToolSession(context.Background())
	ex := NewExecutor(cat, reg, nil, sess, cfg, quiet)
	done := make(chan error, 1)
	go func() {
		_, err := ex.CallTool(context.Background(), readerOnly, "project_list", json.RawMessage(`{}`))
		done <- err
	}()
	<-started
	sess.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error after session close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("call did not stop after session close")
	}
}
