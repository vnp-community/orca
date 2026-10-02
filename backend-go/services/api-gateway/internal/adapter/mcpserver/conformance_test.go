package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
)

const (
	tokenAlice = "tok-alice"
	tokenBob   = "tok-bob"
)

var verifier = mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
	tokenAlice: {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}},
	tokenBob:   {TenantID: "t1", UserID: "bob", Scopes: []string{"orca:read"}},
}}

type fixture struct {
	srv *httptest.Server
	url string // .../mcp
}

// newFixture serves the adapter through chi + otelhttp, exactly like main.go,
// so Flusher/Unwrap regressions in the wrapper chain surface here.
func newFixture(t *testing.T, mutate func(*mcpserver.Deps)) *fixture {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	origins, err := originpolicy.Parse("https://app.example.com")
	if err != nil {
		t.Fatal(err)
	}
	d := mcpserver.Deps{
		Config: mcpserver.Config{
			ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", AllowedOrigins: origins,
			MaxBodyBytes: 4096, PageSize: 2, SessionIdleTTL: time.Minute,
			ScopesSupported: []string{"orca:read"}, ServerVersion: "test",
		},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier:   verifier,
		Catalog:    mcpservertest.FakeCatalog{Names: []string{"a", "b", "c", "d", "e"}},
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
	}
	if mutate != nil {
		mutate(&d)
	}
	r := chi.NewRouter()
	mcpserver.NewHandler(d).Mount(r)
	srv.Config.Handler = otelhttp.NewHandler(r, "test")
	srv.Start()
	t.Cleanup(srv.Close)
	return &fixture{srv: srv, url: base + "/mcp"}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (f *fixture) connect(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	c := mcp.NewClient(&mcp.Implementation{Name: "conformance", Version: "1"}, nil)
	cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: f.url, HTTPClient: &http.Client{Transport: bearerTransport{token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

type rawResp struct {
	*http.Response
	body string
}

func (f *fixture) do(t *testing.T, method, token string, hdr map[string]string, body string) rawResp {
	t.Helper()
	req, err := http.NewRequest(method, f.url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return rawResp{resp, string(b)}
}

// rpcBody extracts the JSON-RPC payload from a JSON or SSE reply.
func rpcBody(t *testing.T, r rawResp) map[string]any {
	t.Helper()
	b := r.body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "text/event-stream") {
		for _, l := range strings.Split(b, "\n") {
			if d, ok := strings.CutPrefix(l, "data: "); ok {
				b = d
				break
			}
		}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(b), &m); err != nil {
		t.Fatalf("not JSON-RPC (%d %q): %v", r.StatusCode, r.body, err)
	}
	return m
}

func rpcErrCode(m map[string]any) int {
	e, _ := m["error"].(map[string]any)
	c, _ := e["code"].(float64)
	return int(c)
}

func initBody(version string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"raw","version":"1"}}}`, version)
}

func TestInitializeAndCapabilities(t *testing.T) {
	f := newFixture(t, nil)
	cs := f.connect(t, tokenAlice)
	res := cs.InitializeResult()
	if res.ProtocolVersion != "2025-06-18" {
		t.Errorf("protocolVersion = %q", res.ProtocolVersion)
	}
	if res.ServerInfo.Name != "orca" || res.ServerInfo.Version != "test" {
		t.Errorf("serverInfo = %+v", res.ServerInfo)
	}
	caps := res.Capabilities
	if caps.Tools == nil || caps.Tools.ListChanged {
		t.Errorf("tools capability must be declared without listChanged: %+v", caps.Tools)
	}
	if caps.Logging == nil {
		t.Error("logging capability must be declared")
	}
	if caps.Resources != nil || caps.Prompts != nil || caps.Completions != nil {
		t.Errorf("must not declare unimplemented capabilities: %+v", caps)
	}
	if res.Instructions == "" {
		t.Error("instructions expected")
	}
}

func TestProtocolVersionNegotiation(t *testing.T) {
	f := newFixture(t, nil)
	for _, v := range []string{"2025-06-18", "1999-01-01", "2026-07-28", "2024-11-05"} {
		r := f.do(t, http.MethodPost, tokenAlice, nil, initBody(v))
		if r.StatusCode != 200 {
			t.Fatalf("%s: status %d %s", v, r.StatusCode, r.body)
		}
		got, _ := rpcBody(t, r)["result"].(map[string]any)["protocolVersion"].(string)
		if got != mcpserver.Negotiate(v) || got != "2025-06-18" {
			t.Errorf("requested %q -> %q", v, got)
		}
	}
	// Unsupported MCP-Protocol-Version header after init -> 400.
	r := f.do(t, http.MethodPost, tokenAlice, map[string]string{"MCP-Protocol-Version": "1999-01-01"},
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("unsupported version header: status %d", r.StatusCode)
	}
}

func TestPing(t *testing.T) {
	f := newFixture(t, nil)
	cs := f.connect(t, tokenAlice)
	if err := cs.Ping(context.Background(), nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestToolsListPagination(t *testing.T) {
	f := newFixture(t, nil)
	cs := f.connect(t, tokenAlice)
	ctx := context.Background()
	var names []string
	cursor := ""
	pages := 0
	for {
		res, err := cs.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		pages++
		for _, tl := range res.Tools {
			names = append(names, tl.Name)
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	if pages != 3 || strings.Join(names, "") != "abcde" {
		t.Errorf("pages=%d names=%v", pages, names)
	}

	// Forged / corrupted cursor -> -32602.
	first, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	for _, bad := range []string{"garbage", first.NextCursor + "x", "e30." + strings.SplitN(first.NextCursor, ".", 2)[1]} {
		_, err := cs.ListTools(ctx, &mcp.ListToolsParams{Cursor: bad})
		var je *jsonrpc.Error
		if !errors.As(err, &je) || je.Code != jsonrpc.CodeInvalidParams {
			t.Errorf("cursor %q: want -32602, got %v", bad, err)
		}
	}

	// A cursor is bound to its session.
	other := f.connect(t, tokenAlice)
	_, err = other.ListTools(ctx, &mcp.ListToolsParams{Cursor: first.NextCursor})
	var je *jsonrpc.Error
	if !errors.As(err, &je) || je.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("cross-session cursor: want -32602, got %v", err)
	}
}

func TestToolsListEmptyCatalogIsEmptyArray(t *testing.T) {
	f := newFixture(t, func(d *mcpserver.Deps) { d.Catalog = nil })
	init := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18"))
	sid := init.Header.Get("Mcp-Session-Id")
	h := map[string]string{"Mcp-Session-Id": sid, "MCP-Protocol-Version": "2025-06-18"}
	if r := f.do(t, http.MethodPost, tokenAlice, h, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); r.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized: %d", r.StatusCode)
	}
	r := f.do(t, http.MethodPost, tokenAlice, h, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if !strings.Contains(r.body, `"tools":[]`) {
		t.Errorf("empty catalog must serialize tools as [], got %s", r.body)
	}
}

func TestToolsListUsesPrincipalPerRequest(t *testing.T) {
	f := newFixture(t, func(d *mcpserver.Deps) {
		d.Catalog = mcpservertest.FakeCatalog{Names: []string{"x"}, ByTenant: map[string][]string{"t1": {"t1-tool"}}}
	})
	res, err := f.connect(t, tokenAlice).ListTools(context.Background(), nil)
	if err != nil || len(res.Tools) != 1 || res.Tools[0].Name != "t1-tool" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestUnauthenticatedChallenge(t *testing.T) {
	f := newFixture(t, nil)
	for name, tok := range map[string]string{"no token": "", "bad token": "nope"} {
		r := f.do(t, http.MethodPost, tok, nil, initBody("2025-06-18"))
		if r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s: status %d", name, r.StatusCode)
		}
		wa := r.Header.Get("WWW-Authenticate")
		want := fmt.Sprintf(`resource_metadata="%s/.well-known/oauth-protected-resource"`, strings.TrimSuffix(f.url, "/mcp"))
		if !strings.HasPrefix(wa, "Bearer ") || !strings.Contains(wa, want) {
			t.Errorf("%s: WWW-Authenticate = %q, want to contain %q", name, wa, want)
		}
		if (tok != "") != strings.Contains(wa, `error="invalid_token"`) {
			t.Errorf("%s: error attr mismatch in %q", name, wa)
		}
	}
}

func TestDefaultVerifierDeniesEverything(t *testing.T) {
	f := newFixture(t, func(d *mcpserver.Deps) { d.Verifier = nil })
	if r := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18")); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", r.StatusCode)
	}
}

func TestOriginPolicy(t *testing.T) {
	f := newFixture(t, nil)
	cases := []struct {
		origin string
		want   int
	}{
		{"", 200}, {"https://app.example.com", 200},
		{"https://evil.example", 403}, {"null", 403}, {"http://app.example.com", 403},
	}
	for _, c := range cases {
		h := map[string]string{}
		if c.origin != "" {
			h["Origin"] = c.origin
		}
		if r := f.do(t, http.MethodPost, tokenAlice, h, initBody("2025-06-18")); r.StatusCode != c.want {
			t.Errorf("Origin %q: status %d want %d", c.origin, r.StatusCode, c.want)
		}
	}
	// Origin check precedes auth: a hostile origin never learns token validity.
	if r := f.do(t, http.MethodPost, "", map[string]string{"Origin": "https://evil.example"}, initBody("2025-06-18")); r.StatusCode != 403 {
		t.Errorf("hostile origin without token: %d", r.StatusCode)
	}
}

func TestOriginRejectedWhenNoAllowListConfigured(t *testing.T) {
	f := newFixture(t, func(d *mcpserver.Deps) { d.Config.AllowedOrigins = nil })
	if r := f.do(t, http.MethodPost, tokenAlice, map[string]string{"Origin": "https://app.example.com"}, initBody("2025-06-18")); r.StatusCode != 403 {
		t.Errorf("status %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18")); r.StatusCode != 200 {
		t.Errorf("no-Origin client must still work: %d", r.StatusCode)
	}
}

func TestCookieAuthRejected(t *testing.T) {
	f := newFixture(t, nil)
	r := f.do(t, http.MethodPost, "", map[string]string{"Cookie": "orca_session=valid-looking-session"}, initBody("2025-06-18"))
	if r.StatusCode != http.StatusUnauthorized || !strings.Contains(r.Header.Get("WWW-Authenticate"), "resource_metadata=") {
		t.Errorf("cookie-only: %d %q", r.StatusCode, r.Header.Get("WWW-Authenticate"))
	}
	// Bearer wins; the cookie is stripped before reaching the verifier.
	var sawCookie bool
	f2 := newFixture(t, func(d *mcpserver.Deps) { d.Verifier = cookieSpy{inner: verifier, saw: &sawCookie} })
	r = f2.do(t, http.MethodPost, tokenAlice, map[string]string{"Cookie": "orca_session=x"}, initBody("2025-06-18"))
	if r.StatusCode != 200 || sawCookie {
		t.Errorf("bearer+cookie: status %d, verifier saw cookie=%v", r.StatusCode, sawCookie)
	}
}

type cookieSpy struct {
	inner mcpserver.TokenVerifier
	saw   *bool
}

func (c cookieSpy) Verify(ctx context.Context, r *http.Request) (mcpserver.Principal, error) {
	if r.Header.Get("Cookie") != "" {
		*c.saw = true
	}
	return c.inner.Verify(ctx, r)
}

func TestRequestBeforeInitializedRejected(t *testing.T) {
	f := newFixture(t, nil)
	// No session at all.
	r := f.do(t, http.MethodPost, tokenAlice, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if r.StatusCode != http.StatusBadRequest || rpcErrCode(rpcBody(t, r)) != -32600 {
		t.Errorf("tools/list without session: %d %s", r.StatusCode, r.body)
	}
	// Session exists but notifications/initialized not yet sent.
	init := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18"))
	h := map[string]string{"Mcp-Session-Id": init.Header.Get("Mcp-Session-Id")}
	r = f.do(t, http.MethodPost, tokenAlice, h, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	m := rpcBody(t, r)
	if m["error"] == nil {
		t.Fatalf("tools/list before initialized must fail: %s", r.body)
	}
	if rpcErrCode(m) != -32600 {
		t.Errorf("want -32600, got %s", r.body)
	}
	// ping is allowed in any state.
	r = f.do(t, http.MethodPost, tokenAlice, h, `{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	if m := rpcBody(t, r); m["error"] != nil {
		t.Errorf("ping before initialized: %s", r.body)
	}
}

func TestBodyLimit(t *testing.T) {
	f := newFixture(t, nil)
	big := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"` + strings.Repeat("x", 8192) + `"}}`
	r := f.do(t, http.MethodPost, tokenAlice, nil, big)
	if r.StatusCode != http.StatusRequestEntityTooLarge || rpcErrCode(rpcBody(t, r)) != -32600 {
		t.Errorf("content-length overflow: %d %s", r.StatusCode, r.body)
	}
	// Chunked (unknown length) overflow is caught by MaxBytesReader.
	pr, pw := io.Pipe()
	go func() { _, _ = pw.Write([]byte(big)); _ = pw.Close() }()
	req, _ := http.NewRequest(http.MethodPost, f.url, pr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+tokenAlice)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("chunked overflow: %d", resp.StatusCode)
	}
}

func TestTransportHeaders(t *testing.T) {
	f := newFixture(t, nil)
	if r := f.do(t, http.MethodPost, tokenAlice, map[string]string{"Content-Type": "text/plain"}, initBody("2025-06-18")); r.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("content-type: %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodPost, tokenAlice, map[string]string{"Accept": "application/json"}, initBody("2025-06-18")); r.StatusCode/100 != 4 {
		t.Errorf("accept without event-stream: %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodGet, tokenAlice, nil, ""); r.StatusCode != http.StatusBadRequest {
		t.Errorf("GET without session: %d", r.StatusCode)
	}
	// Trailing slash must not redirect (POST would lose its body).
	req, _ := http.NewRequest(http.MethodPost, f.url+"/", strings.NewReader(initBody("2025-06-18")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+tokenAlice)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("/mcp/: %d", resp.StatusCode)
	}
}

func TestSessionBoundToUserAndDelete(t *testing.T) {
	f := newFixture(t, nil)
	init := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18"))
	sid := init.Header.Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("no session id")
	}
	h := map[string]string{"Mcp-Session-Id": sid}
	ping := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	if r := f.do(t, http.MethodPost, tokenBob, h, ping); r.StatusCode != http.StatusNotFound {
		t.Errorf("other user's session: %d (want 404, indistinguishable from unknown)", r.StatusCode)
	}
	if r := f.do(t, http.MethodPost, tokenAlice, map[string]string{"Mcp-Session-Id": "unknown"}, ping); r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown session: %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodDelete, tokenBob, h, ""); r.StatusCode != http.StatusNotFound {
		t.Errorf("other user DELETE: %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodDelete, tokenAlice, h, ""); r.StatusCode != http.StatusNoContent {
		t.Errorf("DELETE: %d", r.StatusCode)
	}
	if r := f.do(t, http.MethodPost, tokenAlice, h, ping); r.StatusCode != http.StatusNotFound {
		t.Errorf("after DELETE: %d", r.StatusCode)
	}
}

func TestLoggingSetLevel(t *testing.T) {
	f := newFixture(t, nil)
	cs := f.connect(t, tokenAlice)
	ctx := context.Background()
	if err := cs.SetLoggingLevel(ctx, &mcp.SetLoggingLevelParams{Level: "warning"}); err != nil {
		t.Fatalf("valid level: %v", err)
	}
	err := cs.SetLoggingLevel(ctx, &mcp.SetLoggingLevelParams{Level: "loud"})
	var je *jsonrpc.Error
	if !errors.As(err, &je) || je.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("invalid level: want -32602, got %v", err)
	}
}

type failingExecutor struct{ err error }

func (e failingExecutor) CallTool(context.Context, mcpserver.Principal, string, json.RawMessage) (*mcp.CallToolResult, error) {
	return nil, e.err
}

func TestToolsCallErrorMapping(t *testing.T) {
	ctx := context.Background()
	// No executor: unknown tool is a protocol error.
	cs := newFixture(t, nil).connect(t, tokenAlice)
	_, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "nope"})
	var je *jsonrpc.Error
	if !errors.As(err, &je) || je.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("unknown tool: want -32602, got %v", err)
	}
	// Business failure is a tool result with isError, not a JSON-RPC error.
	f := newFixture(t, func(d *mcpserver.Deps) {
		d.Executor = failingExecutor{err: errors.New("rpc error: code = Internal desc = pq: password authentication failed")}
	})
	res, err := f.connect(t, tokenAlice).CallTool(ctx, &mcp.CallToolParams{Name: "any"})
	if err != nil || !res.IsError {
		t.Fatalf("want isError result, got %+v / %v", res, err)
	}
	txt := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(txt, "rpc error") || strings.Contains(txt, "pq") {
		t.Errorf("internal detail leaked: %q", txt)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	f := newFixture(t, nil)
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		resp, err := http.Get(strings.TrimSuffix(f.url, "/mcp") + path) // no token needed
		if err != nil {
			t.Fatal(err)
		}
		var md struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
			ScopesSupported      []string `json:"scopes_supported"`
			Bearer               []string `json:"bearer_methods_supported"`
		}
		err = json.NewDecoder(resp.Body).Decode(&md)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: %d %v", path, resp.StatusCode, err)
		}
		if md.Resource != f.url || len(md.AuthorizationServers) != 1 || md.AuthorizationServers[0] != "https://auth.example.com" ||
			len(md.ScopesSupported) != 1 || md.Bearer[0] != "header" {
			t.Errorf("%s: %+v", path, md)
		}
	}
}

func TestRateLimitedByTenant(t *testing.T) {
	f := newFixture(t, func(d *mcpserver.Deps) { d.RateLimiter = denyAllLimiter() })
	if r := f.do(t, http.MethodPost, tokenAlice, nil, initBody("2025-06-18")); r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Errorf("status %d", r.StatusCode)
	}
}
