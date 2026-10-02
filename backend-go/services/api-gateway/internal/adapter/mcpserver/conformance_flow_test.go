package mcpserver_test

// Consolidated in-process conformance flow (BE-MCP-SOL-015 tier 1) plus the
// JSON-RPC decode fuzz target. Individual protocol cases live in
// conformance_test.go and session_replicas_test.go; this file strings the
// lifecycle together the way a real client does and adds what no other test
// covers: resources/* and prompts/* through the real handler, a mid-stream cut
// resumed on the OTHER replica, and the fuzz target.
//
// Run all of tier 1 with backend-go/ci/mcp-conformance/run-go-conformance.sh.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

type fakeResources struct{ bound mcpserver.ResourceNotifier }

func (f *fakeResources) ListResources(context.Context, mcpserver.Principal) ([]*mcp.Resource, error) {
	return []*mcp.Resource{{URI: "orca://demo/readme", Name: "readme", MIMEType: "text/plain"}}, nil
}
func (*fakeResources) ListTemplates(context.Context, mcpserver.Principal) ([]*mcp.ResourceTemplate, error) {
	return nil, nil
}
func (*fakeResources) ReadResource(_ context.Context, _ mcpserver.Principal, _ string, uri string) (*mcp.ReadResourceResult, error) {
	if uri != "orca://demo/readme" {
		return nil, mcpserver.ErrResourceNotFound
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/plain", Text: "hello"}}}, nil
}
func (*fakeResources) Subscribe(context.Context, mcpserver.Principal, string, string) error {
	return mcpserver.ErrSubscriptionUnsupported
}
func (*fakeResources) Unsubscribe(string, string)          { /* nothing subscribed */ }
func (*fakeResources) EndSession(string)                   { /* nothing to drop */ }
func (f *fakeResources) Bind(n mcpserver.ResourceNotifier) { f.bound = n }
func (*fakeResources) Subscribable() bool                  { return false }

type fakePrompts struct{}

func (fakePrompts) ListPrompts(context.Context, mcpserver.Principal, string) ([]*mcp.Prompt, error) {
	return []*mcp.Prompt{{Name: "review", Description: "review a diff"}}, nil
}
func (fakePrompts) GetPrompt(_ context.Context, _ mcpserver.Principal, _ string, name string, _ map[string]string, _ string) (*mcp.GetPromptResult, error) {
	if name != "review" {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "unknown prompt"}
	}
	return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: "please review"}}}}, nil
}

func TestConformanceFlow(t *testing.T) {
	cl := newCluster(t, 0)
	release := make(chan struct{})
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, name string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		switch name {
		case "slow":
			for i := 0; i < 10; i++ {
				cl.clk.Advance(250 * time.Millisecond)
				mcpserver.ReportProgress(ctx, float64(i), 10, "")
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return okResult("done")
		case "fail":
			return nil, errTool("MCP_POLICY_DENIED: no")
		}
		return okResult("pong:" + name)
	})
	mutate := func(d *mcpserver.Deps) {
		d.Resources, d.Prompts = &fakeResources{}, fakePrompts{}
		d.Catalog = mcpservertest.FakeCatalog{Names: []string{"echo", "slow", "fail"}}
	}
	a, b := cl.replica(t, exec, mutate), cl.replica(t, exec, mutate)

	// Lifecycle through the official Go client: initialize, version
	// negotiation, ping, tools, resources, prompts.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "flow", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: a.url, HTTPClient: &http.Client{Transport: bearerTransport{tokenAlice}}}, nil)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer cs.Close()
	init := cs.InitializeResult()
	if init.ServerInfo.Name != "orca" || init.Capabilities.Tools == nil || init.ProtocolVersion == "" {
		t.Fatalf("initialize result: %+v", init)
	}
	if err := cs.Ping(ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) == 0 {
		t.Fatalf("tools/list: %v %v", tools, err)
	}
	if res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{}}); err != nil || res.IsError {
		t.Fatalf("tools/call echo: %v %v", res, err)
	}
	// A business failure is a tool result (isError), never a JSON-RPC error.
	if res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "fail", Arguments: map[string]any{}}); err != nil || !res.IsError {
		t.Fatalf("tools/call fail must be isError without a protocol error: %v %v", res, err)
	}
	rl, err := cs.ListResources(ctx, nil)
	if err != nil || len(rl.Resources) != 1 {
		t.Fatalf("resources/list: %v %v", rl, err)
	}
	if rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "orca://demo/readme"}); err != nil || rr.Contents[0].Text != "hello" {
		t.Fatalf("resources/read: %v %v", rr, err)
	}
	// "Unknown", "not yours" and "denied" are one answer: -32002/invalid params, no oracle.
	if _, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "orca://demo/other"}); err == nil {
		t.Fatal("resources/read of an unknown URI must fail")
	}
	pl, err := cs.ListPrompts(ctx, nil)
	if err != nil || len(pl.Prompts) != 1 {
		t.Fatalf("prompts/list: %v %v", pl, err)
	}
	if pg, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "review"}); err != nil || len(pg.Messages) != 1 {
		t.Fatalf("prompts/get: %v %v", pg, err)
	}
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "nope"}); err == nil {
		t.Fatal("prompts/get of an unknown prompt must be a protocol error")
	}

	// Resumability: cut a streamed call after 4 events on A, resume on B.
	sid := a.initSession(t, tokenAlice)
	cctx, cut := context.WithCancel(ctx)
	resp := a.req(t, cctx, http.MethodPost, tokenAlice, sid, callBody(9, "p"), nil)
	first := readSSE(resp.Body, 4)
	cut()
	_ = resp.Body.Close()
	if len(first) != 4 {
		t.Fatalf("first leg: %d events", len(first))
	}
	got := b.req(t, ctx, http.MethodGet, tokenAlice, sid, "", map[string]string{"Last-Event-ID": first[3].ID})
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("resume on the other replica: %d", got.StatusCode)
	}
	close(release)
	rest := readSSE(got.Body, 0)
	if len(rest) == 0 || !strings.Contains(rest[len(rest)-1].Data, `"id":9`) {
		t.Fatalf("resumed stream must end with the response to request 9: %+v", rest)
	}
	// A Last-Event-ID that was never issued is a clean 404, not a hang or 500.
	bad := b.req(t, ctx, http.MethodGet, tokenAlice, sid, "", map[string]string{"Last-Event-ID": "nonsense"})
	_ = bad.Body.Close()
	if bad.StatusCode >= 500 {
		t.Fatalf("bogus Last-Event-ID: %d", bad.StatusCode)
	}

	// Auth negatives on the same stack.
	for name, tc := range map[string]struct {
		token string
		hdr   map[string]string
		want  int
	}{
		"no token":        {"", nil, http.StatusUnauthorized},
		"unknown token":   {"tok-nobody", nil, http.StatusUnauthorized},
		"cookie only":     {"", map[string]string{"Cookie": "orca_session=abc"}, http.StatusUnauthorized},
		"foreign origin":  {tokenAlice, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		"other user sess": {tokenBob, map[string]string{"Mcp-Session-Id": sid}, http.StatusNotFound},
	} {
		r := a.req(t, ctx, http.MethodPost, tc.token, "", `{"jsonrpc":"2.0","id":1,"method":"ping"}`, tc.hdr)
		_ = r.Body.Close()
		if r.StatusCode != tc.want {
			t.Errorf("%s: %d, want %d", name, r.StatusCode, tc.want)
		}
	}
}

type errTool string

func (e errTool) Error() string { return string(e) }

// FuzzJSONRPCDecode feeds arbitrary bytes to the SDK decoder and, through a
// live session, to POST /mcp. Properties: no panic, never a 5xx, and an
// authenticated, initialized session survives garbage.
func FuzzJSONRPCDecode(f *testing.F) {
	for _, seed := range []string{
		``, `{}`, `[]`, `null`, `{"jsonrpc":"2.0"}`, `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":"x","method":"tools/list","params":{"cursor":"%%%"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"a","arguments":[1,2]}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":123}}`,
		`[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`,
		`{"jsonrpc":"1.0","id":1,"method":"ping"}`, `{"jsonrpc":"2.0","id":1e999,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"\u0000"}}`,
		`{"jsonrpc":"2.0","id":1,"method":"logging/setLevel","params":{"level":{}}}`,
		`{"jsonrpc":"2.0","id":1,"method":"notifications/cancelled","params":{"requestId":{}}}`,
		"\xff\xfe\x00{", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"x"}}`,
	} {
		f.Add([]byte(seed))
	}
	h := mcpserver.NewHandler(mcpserver.Deps{
		Config:     mcpserver.Config{ResourceURL: "http://example.test/mcp", IssuerURL: "https://auth.example.com", MaxBodyBytes: 1 << 14, SessionIdleTTL: time.Minute},
		Verifier:   verifier,
		Catalog:    mcpservertest.FakeCatalog{Names: []string{"a"}},
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
	})
	f.Cleanup(h.Close)
	r := chi.NewRouter()
	h.Mount(r)
	post := func(sid string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+tokenAlice)
		if sid != "" {
			req.Header.Set("Mcp-Session-Id", sid)
			req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	rec := post("", []byte(initBody("2025-06-18")))
	sid := rec.Header().Get("Mcp-Session-Id")
	if sid == "" {
		f.Fatalf("setup: initialize failed: %d %s", rec.Code, rec.Body.String())
	}
	post(sid, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = jsonrpc.DecodeMessage(data) // must not panic whatever the bytes
		for _, session := range []string{"", sid} {
			if got := post(session, data); got.Code >= 500 {
				t.Fatalf("session=%q: %d for %q -> %s", session, got.Code, data, io.LimitReader(got.Body, 200))
			}
		}
		if got := post(sid, []byte(`{"jsonrpc":"2.0","id":99,"method":"ping"}`)); got.Code != http.StatusOK {
			t.Fatalf("session no longer serves ping after %q: %d", data, got.Code)
		}
	})
}
