package mcpprober

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// fakeResolver answers per call number so tests can flip the answer
// (DNS rebinding) between the first and later lookups.
type fakeResolver struct {
	mu    sync.Mutex
	calls int
	pick  func(call int) []netip.Addr
}

func (f *fakeResolver) LookupNetIP(_ context.Context, _, _ string) ([]netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.pick(f.calls), nil
}

func fixed(ips ...string) *fakeResolver {
	var as []netip.Addr
	for _, s := range ips {
		as = append(as, netip.MustParseAddr(s))
	}
	return &fakeResolver{pick: func(int) []netip.Addr { return as }}
}

// mcpServer is a minimal Streamable HTTP MCP server.
type mcpServer struct {
	hits    atomic.Int32
	headers []http.Header
	mu      sync.Mutex
	handler func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool
}

func (m *mcpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.hits.Add(1)
	m.mu.Lock()
	m.headers = append(m.headers, r.Header.Clone())
	m.mu.Unlock()
	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var msg map[string]any
	_ = json.NewDecoder(r.Body).Decode(&msg)
	if m.handler != nil && m.handler(w, r, msg) {
		return
	}
	switch msg["method"] {
	case "initialize":
		w.Header().Set("Mcp-Session-Id", "sess-1")
		reply(w, msg, map[string]any{"protocolVersion": "2025-06-18"})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		reply(w, msg, map[string]any{"tools": []map[string]any{{"name": "echo", "description": "echoes"}}})
	}
}

func reply(w http.ResponseWriter, msg map[string]any, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": result})
}

type env struct {
	srv  *mcpServer
	ts   *httptest.Server
	url  string
	port int
}

func newEnv(t *testing.T) env {
	t.Helper()
	s := &mcpServer{}
	ts := httptest.NewTLSServer(s)
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)
	var port int
	fmt.Sscan(u.Port(), &port)
	return env{srv: s, ts: ts, port: port, url: fmt.Sprintf("https://mcp.example.test:%d/mcp", port)}
}

func (e env) prober(r Resolver, blocked func(netip.Addr) bool) *Prober {
	return New(Config{
		Policy: domain.ExternalURLPolicy{AllowedPorts: []int{e.port}}, Resolver: r, Blocked: blocked,
		TLSConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // test server cert
	})
}

func allowOnlyLoopback(ip netip.Addr) bool { return !ip.Unmap().IsLoopback() }

func isCode(err error, code string) bool { return err != nil && strings.Contains(err.Error(), code) }

func TestProber_HappyPathSendsOnlyConfiguredHeaders(t *testing.T) {
	e := newEnv(t)
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	res, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url, Headers: map[string]domain.SecretValue{
		"X-Api-Key": domain.NewSecretValue("k-123"), "Host": domain.NewSecretValue("evil"), "Cookie": domain.NewSecretValue("c"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 1 || res.Tools[0].Name != "echo" {
		t.Fatalf("tools: %+v", res.Tools)
	}
	for _, h := range e.srv.headers {
		if h.Get("X-Api-Key") != "k-123" {
			t.Errorf("configured header missing: %v", h)
		}
		if h.Get("Authorization") != "" || h.Get("Cookie") != "" {
			t.Errorf("unexpected credential header: %v", h)
		}
	}
	if e.srv.headers[len(e.srv.headers)-1].Get("Mcp-Session-Id") != "sess-1" {
		t.Error("session id must be echoed so the session can be closed")
	}
}

func TestProber_NeverForwardsCallerToken(t *testing.T) {
	// The probe target has no field for a caller identity and the context's
	// metadata is not read; this asserts no Authorization header goes out
	// unless a header reference supplied it.
	e := newEnv(t)
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	ctx := context.WithValue(context.Background(), struct{}{}, "Bearer caller-token")
	if _, err := p.ListTools(ctx, usecase.ProbeTarget{URL: e.url}); err != nil {
		t.Fatal(err)
	}
	for _, h := range e.srv.headers {
		if h.Get("Authorization") != "" {
			t.Fatalf("Authorization leaked: %v", h)
		}
	}
}

func TestProber_SSRFTable_BlockedBeforeAnyRequest(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.1.2.3", "172.20.0.5", "192.168.0.9",
		"100.64.0.7", "100.100.100.200", "169.254.169.254", "fd00:ec2::254", "0.0.0.0", "fe80::1",
	} {
		e := newEnv(t)
		p := e.prober(fixed(ip), nil) // real domain.IsBlockedIP
		_, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url})
		if !isCode(err, domain.CodeServerSSRFBlocked) {
			t.Errorf("%s: want %s, got %v", ip, domain.CodeServerSSRFBlocked, err)
		}
		if e.srv.hits.Load() != 0 {
			t.Errorf("%s: server was contacted", ip)
		}
	}
}

func TestProber_MixedAnswerWithOneBlockedAddressIsRejected(t *testing.T) {
	e := newEnv(t)
	p := e.prober(fixed("93.184.216.34", "169.254.169.254"), nil)
	if _, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url}); !isCode(err, domain.CodeServerSSRFBlocked) {
		t.Fatalf("got %v", err)
	}
}

func TestProber_DNSRebindingIsDefeatedAtDialTime(t *testing.T) {
	e := newEnv(t)
	// First lookup: an address the (test) policy allows. Every later lookup:
	// the metadata address. A check-then-connect design that resolves once
	// would succeed for all requests; ours re-checks per connection.
	flip := &fakeResolver{pick: func(call int) []netip.Addr {
		if call == 1 {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}
		}
		return []netip.Addr{netip.MustParseAddr("169.254.169.254")}
	}}
	p := e.prober(flip, allowOnlyLoopback)
	_, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url})
	if !isCode(err, domain.CodeServerSSRFBlocked) {
		t.Fatalf("want SSRF block after flip, got %v", err)
	}
	if e.srv.hits.Load() != 1 {
		t.Fatalf("only the first (pre-flip) request may reach the server, got %d", e.srv.hits.Load())
	}
}

func TestProber_RedirectToPrivateIsNotFollowed(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data", http.StatusFound)
		return true
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	_, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url})
	var pf domain.ErrProbeFailed
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("want redirect refusal, got %v (%v)", err, pf)
	}
	if e.srv.hits.Load() != 1 {
		t.Fatalf("hits=%d", e.srv.hits.Load())
	}
}

func TestProber_OversizedBodyIsRejected(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"pad":"%s"}}`, strings.Repeat("a", MaxBodyBytes+10))
			return true
		}
		return false
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	if _, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url}); err == nil || !strings.Contains(err.Error(), "bad_response") {
		t.Fatalf("got %v", err)
	}
}

func TestProber_SSEAndPagination(t *testing.T) {
	e := newEnv(t)
	e.srv.handler = func(w http.ResponseWriter, r *http.Request, msg map[string]any) bool {
		if msg["method"] != "tools/list" {
			return false
		}
		params, _ := msg["params"].(map[string]any)
		w.Header().Set("Content-Type", "text/event-stream")
		next, name := `"p2"`, "one"
		if params["cursor"] == "p2" {
			next, name = "null", "two"
		}
		if next == "null" {
			next = `""`
		}
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":%v,\"result\":{\"tools\":[{\"name\":%q,\"description\":\"d\"}],\"nextCursor\":%s}}\n\n", msg["id"], name, next)
		return true
	}
	p := e.prober(fixed("127.0.0.1"), allowOnlyLoopback)
	res, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url})
	if err != nil || len(res.Tools) != 2 {
		t.Fatalf("tools=%v err=%v", res.Tools, err)
	}
}

func TestProber_ErrorMessagesNeverContainResolvedAddress(t *testing.T) {
	e := newEnv(t)
	p := e.prober(fixed("169.254.169.254"), nil)
	_, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: e.url})
	if err == nil || strings.Contains(err.Error(), "169.254") {
		t.Fatalf("error must not echo the address: %v", err)
	}
}

func TestProber_StaticURLValidationRunsFirst(t *testing.T) {
	p := New(Config{Resolver: fixed("8.8.8.8")})
	for _, u := range []string{"http://x.example.com/", "https://10.0.0.1/", "https://u:p@x.example.com/"} {
		if _, err := p.ListTools(context.Background(), usecase.ProbeTarget{URL: u}); err == nil {
			t.Errorf("%s must be rejected", u)
		}
	}
}
