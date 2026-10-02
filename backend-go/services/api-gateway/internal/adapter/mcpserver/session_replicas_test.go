package mcpserver_test

// Two-replica tests (BE-MCP-SOL-004). The "replicas" are two independent
// mcpserver.Handlers behind two httptest servers that share ONLY what real
// replicas share: the session store (stands in for mcp-service), the resume
// buffer (stands in for the JetStream MCPSSE stream) and the signal bus (stands
// in for core NATS). They are in-process fakes; the same flows against real
// NATS are in mcpsession's integration test (-tags=integration).

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/goleak"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

type fakeClock struct{ ns atomic.Int64 }

func newFakeClock() *fakeClock {
	c := &fakeClock{}
	c.ns.Store(time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC).UnixNano())
	return c
}
func (c *fakeClock) Now() time.Time          { return time.Unix(0, c.ns.Load()).UTC() }
func (c *fakeClock) Advance(d time.Duration) { c.ns.Add(int64(d)) }

type cluster struct {
	clk    *fakeClock
	store  *mcpserver.MemorySessionStore
	events *mcpserver.MemoryResumableStore
	bus    *mcpserver.LocalSignalBus
	rec    *mcpservertest.CountingRecorder
}

func newCluster(t *testing.T, maxEvents int) *cluster {
	t.Helper()
	clk := newFakeClock()
	return &cluster{clk: clk, store: mcpserver.NewMemorySessionStore(time.Minute, clk.Now),
		events: mcpserver.NewMemoryResumableStore(maxEvents, 0), bus: mcpserver.NewLocalSignalBus(), rec: &mcpservertest.CountingRecorder{}}
}

type replica struct {
	h   *mcpserver.Handler
	srv *httptest.Server
	url string
}

type execFunc func(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error)

func (f execFunc) CallTool(ctx context.Context, p mcpserver.Principal, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	return f(ctx, p, name, args)
}

func (c *cluster) replica(t *testing.T, exec mcpserver.ToolExecutor, mutate func(*mcpserver.Deps)) *replica {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	d := mcpserver.Deps{
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://auth.example.com", MaxBodyBytes: 1 << 16,
			SessionIdleTTL: time.Minute, ServerVersion: "test", MaxStreamsPerUser: 5, MaxStreamsPerTenant: 50},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Verifier: verifier, Catalog: mcpservertest.FakeCatalog{Names: []string{"a"}},
		CursorKeys: [][]byte{[]byte("k1-0123456789abcdef0123456789abcd")},
		Executor:   exec, Sessions: c.store, EventStore: c.events, Signals: c.bus, Recorder: c.rec, Now: c.clk.Now,
	}
	if mutate != nil {
		mutate(&d)
	}
	h := mcpserver.NewHandler(d)
	r := chi.NewRouter()
	h.Mount(r)
	srv.Config.Handler = r
	srv.Start()
	rep := &replica{h: h, srv: srv, url: base + "/mcp"}
	t.Cleanup(func() { rep.close() })
	return rep
}

func (r *replica) close() {
	r.srv.CloseClientConnections()
	r.srv.Close()
	r.h.Close()
}

func (r *replica) req(t *testing.T, ctx context.Context, method, token, sid, body string, hdr map[string]string) *http.Response {
	t.Helper()
	resp, err := r.reqErr(ctx, method, token, sid, body, hdr)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// reqErr is safe in goroutines (no t.Fatal) and for requests the test cancels.
func (r *replica) reqErr(ctx context.Context, method, token, sid, body string, hdr map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

type sseEvent struct{ ID, Data string }

// readSSE reads up to n events (all, if n<=0) and returns at EOF/cancel.
func readSSE(r io.Reader, n int) []sseEvent {
	var out []sseEvent
	var cur sseEvent
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return out
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "id: "):
			cur.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			cur.Data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if cur.Data != "" {
				out = append(out, cur)
				if n > 0 && len(out) >= n {
					return out
				}
			}
			cur = sseEvent{}
		}
	}
}

func (r *replica) initSession(t *testing.T, token string) string {
	t.Helper()
	resp := r.req(t, context.Background(), http.MethodPost, token, "", initBody("2025-06-18"), nil)
	sid := resp.Header.Get("Mcp-Session-Id")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || sid == "" {
		t.Fatalf("initialize: %d sid=%q", resp.StatusCode, sid)
	}
	ack := r.req(t, context.Background(), http.MethodPost, token, sid, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	_ = ack.Body.Close()
	if ack.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized: %d", ack.StatusCode)
	}
	return sid
}

func callBody(id int, token string) string {
	meta := ""
	if token != "" {
		meta = fmt.Sprintf(`,"_meta":{"progressToken":%q}`, token)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"slow","arguments":{}%s}}`, id, meta)
}

func eventIdx(t *testing.T, id string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(id[strings.LastIndex(id, "_")+1:], "%d", &n); err != nil {
		t.Fatalf("bad event id %q", id)
	}
	return n
}

func progressValue(t *testing.T, data string) int {
	t.Helper()
	var m struct {
		Params struct {
			Progress float64 `json:"progress"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	return int(m.Params.Progress)
}

func okResult(text string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
}

// POST on replica A emits 100 progress events and is cut after event 39; the
// client resumes on replica B with Last-Event-ID and must receive exactly
// events 40..99 and then the final response (id 100) - no loss, no repeat.
func TestTwoReplicaResume(t *testing.T) {
	cl := newCluster(t, 0)
	release := make(chan struct{})
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		for i := 0; i < 100; i++ {
			cl.clk.Advance(250 * time.Millisecond) // every report passes the 5/s throttle
			mcpserver.ReportProgress(ctx, float64(i), 100, "")
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return okResult("done")
	})
	a, b := cl.replica(t, exec, nil), cl.replica(t, exec, nil)
	sid := a.initSession(t, tokenAlice)

	ctxA, cut := context.WithCancel(context.Background())
	resp := a.req(t, ctxA, http.MethodPost, tokenAlice, sid, callBody(7, "p1"), nil)
	first := readSSE(resp.Body, 40)
	cut() // connection lost mid-stream
	_ = resp.Body.Close()
	if len(first) != 40 || eventIdx(t, first[39].ID) != 39 {
		t.Fatalf("first leg: %d events, last id %q", len(first), first[len(first)-1].ID)
	}
	lastID := first[39].ID

	rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer rcancel()
	got := b.req(t, rctx, http.MethodGet, tokenAlice, sid, "", map[string]string{"Last-Event-ID": lastID})
	defer got.Body.Close()
	if got.StatusCode != 200 || !strings.HasPrefix(got.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("resume on B: %d %q", got.StatusCode, got.Header.Get("Content-Type"))
	}
	rest := readSSE(got.Body, 60)
	for i, e := range rest {
		if eventIdx(t, e.ID) != 40+i || progressValue(t, e.Data) != 40+i {
			t.Fatalf("replayed event %d: id=%s data=%s", i, e.ID, e.Data)
		}
	}
	if len(rest) != 60 {
		t.Fatalf("want 60 replayed events, got %d", len(rest))
	}
	close(release) // the tool (still running on A) finishes: B delivers the response live
	tail := readSSE(got.Body, 0)
	if len(tail) != 1 || eventIdx(t, tail[0].ID) != 100 || !strings.Contains(tail[0].Data, `"id":7`) || !strings.Contains(tail[0].Data, "done") {
		t.Fatalf("tail after release: %+v", tail)
	}
	if cl.rec.Resumes("ok") != 1 {
		t.Errorf("resume metric: %d", cl.rec.Resumes("ok"))
	}
}

// A session created on A is usable on B (adoption from the shared store).
func TestSessionAdoptedByOtherReplica(t *testing.T) {
	cl := newCluster(t, 0)
	a, b := cl.replica(t, nil, nil), cl.replica(t, nil, nil)
	sid := a.initSession(t, tokenAlice)
	resp := b.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"name":"a"`) {
		t.Fatalf("tools/list via B: %d %s", resp.StatusCode, body)
	}
}

func TestSessionIdentityBindingAcrossReplicas(t *testing.T) {
	cl := newCluster(t, 0)
	a, b := cl.replica(t, nil, nil), cl.replica(t, nil, nil)
	sid := a.initSession(t, tokenAlice)
	for name, rep := range map[string]*replica{"A": a, "B": b} {
		for _, m := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
			body := ""
			if m == http.MethodPost {
				body = `{"jsonrpc":"2.0","id":2,"method":"ping"}`
			}
			resp := rep.req(t, context.Background(), m, tokenBob, sid, body, nil)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s with another user's token: %d, want 404", name, m, resp.StatusCode)
			}
		}
	}
	if cl.rec.Mismatch.Load() < 6 {
		t.Errorf("identity_mismatch counter = %d", cl.rec.Mismatch.Load())
	}
	// The owner's session survived the foreign attempts.
	resp := b.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","id":3,"method":"ping"}`, nil)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("owner after foreign attempts: %d", resp.StatusCode)
	}
}

// DELETE closes the session and cancels running tools, also when the tool runs
// on another replica; no goroutine outlives the handlers.
func TestDeleteCancelsToolsNoLeak(t *testing.T) {
	http.DefaultTransport.(*http.Transport).CloseIdleConnections()
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent(), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"))

	cl := newCluster(t, 0)
	running, cancelled := make(chan struct{}, 4), make(chan struct{}, 4)
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		running <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	})
	a, b := cl.replica(t, exec, nil), cl.replica(t, exec, nil)

	for name, deleteOn := range map[string]*replica{"same replica": a, "other replica": b} {
		sid := a.initSession(t, tokenAlice)
		pctx, pcancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			if resp, err := a.reqErr(pctx, http.MethodPost, tokenAlice, sid, callBody(9, ""), nil); err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
		<-running
		del := deleteOn.req(t, context.Background(), http.MethodDelete, tokenAlice, sid, "", nil)
		_ = del.Body.Close()
		if del.StatusCode != http.StatusNoContent {
			t.Fatalf("%s: DELETE %d", name, del.StatusCode)
		}
		select {
		case <-cancelled:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: tool context was not cancelled by DELETE", name)
		}
		pcancel()
		<-done
		after := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","id":3,"method":"ping"}`, nil)
		_ = after.Body.Close()
		if after.StatusCode != http.StatusNotFound {
			t.Errorf("%s: request after DELETE: %d", name, after.StatusCode)
		}
	}
	cl.bus.Wait()
	a.close()
	b.close()
	http.DefaultTransport.(*http.Transport).CloseIdleConnections()
}

func TestIdleTTLEndsSession(t *testing.T) {
	cl := newCluster(t, 0)
	a := cl.replica(t, nil, nil)
	sid := a.initSession(t, tokenAlice)
	ping := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	cl.clk.Advance(30 * time.Second)
	if r := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, ping, nil); r.StatusCode != 200 {
		t.Fatalf("within TTL: %d", r.StatusCode)
	}
	cl.clk.Advance(61 * time.Second) // idle past the TTL
	r := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, ping, nil)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("after idle TTL: %d, want 404", r.StatusCode)
	}
	if cl.rec.ClosedFor("idle") == 0 {
		t.Error("session close not counted")
	}
	if sid2 := a.initSession(t, tokenAlice); sid2 == "" || sid2 == sid {
		t.Fatalf("re-initialize must create a new session, got %q", sid2)
	}
}

// notifications/cancelled cancels the request context - also when the
// notification arrives on a different replica than the one running the tool.
func TestCancelledNotificationCancelsRequest(t *testing.T) {
	cl := newCluster(t, 0)
	running, cancelled := make(chan struct{}, 2), make(chan struct{}, 2)
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		running <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	})
	a, b := cl.replica(t, exec, nil), cl.replica(t, exec, nil)
	for name, sendTo := range map[string]*replica{"same replica": a, "other replica": b} {
		sid := a.initSession(t, tokenAlice)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			if resp, err := a.reqErr(ctx, http.MethodPost, tokenAlice, sid, callBody(11, ""), nil); err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
		<-running
		n := sendTo.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":11,"reason":"user"}}`, nil)
		_ = n.Body.Close()
		if n.StatusCode != http.StatusAccepted {
			t.Fatalf("%s: cancelled notification: %d", name, n.StatusCode)
		}
		select {
		case <-cancelled:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: request context not cancelled", name)
		}
		cancel()
	}
}

// 100 progress reports are coalesced to <=5/s per token and the newest value is
// always delivered before the response.
func TestProgressIsThrottledAndLastValueDelivered(t *testing.T) {
	cl := newCluster(t, 0)
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		for i := 0; i < 100; i++ {
			cl.clk.Advance(20 * time.Millisecond) // 50 reports per (fake) second
			mcpserver.ReportProgress(ctx, float64(i), 100, "")
		}
		return okResult("done")
	})
	a := cl.replica(t, exec, nil)
	sid := a.initSession(t, tokenAlice)
	resp := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, callBody(5, "tok"), nil)
	defer resp.Body.Close()
	evs := readSSE(resp.Body, 0)
	var progress []int
	for _, e := range evs[:len(evs)-1] {
		progress = append(progress, progressValue(t, e.Data))
	}
	if !strings.Contains(evs[len(evs)-1].Data, `"id":5`) {
		t.Fatalf("last event must be the response: %s", evs[len(evs)-1].Data)
	}
	// 100 reports span 2s of fake time: at most 5/s -> ~10 plus the final flush.
	if len(progress) > 12 || len(progress) < 5 {
		t.Errorf("progress events = %d, want a throttled number (<=12)", len(progress))
	}
	if progress[len(progress)-1] != 99 {
		t.Errorf("the newest progress value must be flushed before the response, got %d", progress[len(progress)-1])
	}
	for i := 1; i < len(progress); i++ {
		if progress[i] <= progress[i-1] {
			t.Errorf("progress must stay monotonic: %v", progress)
			break
		}
	}
	if cl.rec.Dropped.Load() == 0 {
		t.Error("coalesced reports not counted")
	}
}

// With a tiny buffer a resume that needs dropped events is a 404 (client
// re-initializes) - bounded memory instead of an unbounded backlog.
func TestResumeGapIs404(t *testing.T) {
	cl := newCluster(t, 5)
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		for i := 0; i < 30; i++ {
			cl.clk.Advance(250 * time.Millisecond)
			mcpserver.ReportProgress(ctx, float64(i), 30, "")
		}
		return okResult("done")
	})
	a, b := cl.replica(t, exec, nil), cl.replica(t, exec, nil)
	sid := a.initSession(t, tokenAlice)
	resp := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, callBody(3, "p"), nil)
	evs := readSSE(resp.Body, 0)
	_ = resp.Body.Close()
	streamID := evs[0].ID[:strings.LastIndex(evs[0].ID, "_")]
	r := b.req(t, context.Background(), http.MethodGet, tokenAlice, sid, "", map[string]string{"Last-Event-ID": streamID + "_2"})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("resume past the bounded buffer: %d, want 404", r.StatusCode)
	}
	if cl.rec.Resumes("gap") != 1 {
		t.Errorf("gap metric: %d", cl.rec.Resumes("gap"))
	}
	// A completed stream resumed after its last event is an empty, finished stream.
	last := fmt.Sprintf("%s_%d", streamID, eventIdx(t, evs[len(evs)-1].ID))
	done := b.req(t, context.Background(), http.MethodGet, tokenAlice, sid, "", map[string]string{"Last-Event-ID": last})
	body, _ := io.ReadAll(done.Body)
	_ = done.Body.Close()
	if done.StatusCode != 200 || strings.Contains(string(body), "data:") {
		t.Errorf("resume after the final response: %d %q", done.StatusCode, body)
	}
}

// Stream caps are cluster-wide when the store is a StreamRegistry.
func TestStreamLimitClusterWide(t *testing.T) {
	cl := newCluster(t, 0)
	mut := func(d *mcpserver.Deps) { d.Config.MaxStreamsPerUser = 1 }
	a, b := cl.replica(t, nil, mut), cl.replica(t, nil, mut)
	sid := a.initSession(t, tokenAlice)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := a.req(t, ctx, http.MethodGet, tokenAlice, sid, "", nil)
	defer first.Body.Close()
	if first.StatusCode != 200 {
		t.Fatalf("first stream: %d", first.StatusCode)
	}
	second := b.req(t, context.Background(), http.MethodGet, tokenAlice, sid, "", nil)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second stream on the OTHER replica: %d, want 429", second.StatusCode)
	}
}

func TestKillSwitchIs403NotInvalidToken(t *testing.T) {
	cl := newCluster(t, 0)
	a := cl.replica(t, nil, func(d *mcpserver.Deps) {
		d.Verifier = killedVerifier{inner: verifier}
	})
	r := a.req(t, context.Background(), http.MethodPost, tokenAlice, "", initBody("2025-06-18"), nil)
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "MCP_KILL_SWITCH_ACTIVE") {
		t.Fatalf("tenant kill switch: %d %s", r.StatusCode, body)
	}
	if wa := r.Header.Get("WWW-Authenticate"); wa != "" {
		t.Errorf("must not challenge (a re-login cannot help): %q", wa)
	}

	// Session-scope kill: keyed by the non-secret row id, checked per request.
	var blocked atomic.Bool
	var seenRow atomic.Value
	b := cl.replica(t, nil, func(d *mcpserver.Deps) {
		d.KillCheck = func(_ context.Context, _ mcpserver.Principal, row string) (bool, error) {
			seenRow.Store(row)
			return blocked.Load(), nil
		}
	})
	sid := b.initSession(t, tokenAlice)
	blocked.Store(true)
	r2 := b.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","id":2,"method":"ping"}`, nil)
	body2, _ := io.ReadAll(r2.Body)
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusForbidden || !strings.Contains(string(body2), "MCP_KILL_SWITCH_ACTIVE") {
		t.Fatalf("session kill switch: %d %s", r2.StatusCode, body2)
	}
	if row, _ := seenRow.Load().(string); row == "" || row == sid {
		t.Errorf("kill check must be keyed by the non-secret row id, got %q", row)
	}
}

type killedVerifier struct{ inner mcpserver.TokenVerifier }

func (k killedVerifier) Verify(ctx context.Context, r *http.Request) (mcpserver.Principal, error) {
	if _, err := k.inner.Verify(ctx, r); err != nil {
		return mcpserver.Principal{}, err
	}
	return mcpserver.Principal{}, mcpserver.ErrKillSwitchActive
}

// Depth/root (verified token claims), the non-secret session id and the client
// name reach the executor's context through Deps.RequestContext.
func TestRequestContextPlumbing(t *testing.T) {
	cl := newCluster(t, 0)
	type seen struct {
		info mcpserver.RequestInfo
		p    mcpserver.Principal
	}
	got := make(chan seen, 1)
	exec := execFunc(func(ctx context.Context, p mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		return okResult("ok")
	})
	a := cl.replica(t, exec, func(d *mcpserver.Deps) {
		d.Verifier = mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
			"agent": {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}, Depth: 2, Root: "root-session"},
		}}
		d.RequestContext = func(ctx context.Context, p mcpserver.Principal, info mcpserver.RequestInfo) context.Context {
			got <- seen{info: info, p: p}
			return ctx
		}
	})
	sid := a.initSession(t, "agent")
	resp := a.req(t, context.Background(), http.MethodPost, "agent", sid, callBody(2, ""), nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	s := <-got
	if s.info.Depth != 2 || s.info.Root != "root-session" {
		t.Errorf("depth/root: %+v", s.info)
	}
	if s.info.ClientName != "raw" {
		t.Errorf("client name: %q", s.info.ClientName)
	}
	if s.info.SessionID == "" || s.info.SessionID == sid {
		t.Errorf("session id must be the non-secret row id, got %q (secret %q)", s.info.SessionID, sid)
	}
}

// The SDK client declares elicitation; the executor context offers an Elicitor
// only then, and the user's answer comes back through the SSE stream.
func TestElicitorOnlyWhenClientSupportsIt(t *testing.T) {
	cl := newCluster(t, 0)
	exec := execFunc(func(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
		el, ok := mcpserver.ElicitorFromContext(ctx)
		if !ok {
			return okResult("no-elicitation")
		}
		d, err := el(ctx, "Approve test action?")
		if err != nil {
			return nil, err
		}
		return okResult(fmt.Sprintf("%s:%v", d.Action, d.Approve))
	})
	a := cl.replica(t, exec, nil)
	run := func(opts *mcp.ClientOptions) string {
		c := mcp.NewClient(&mcp.Implementation{Name: "el", Version: "1"}, opts)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: a.url, HTTPClient: &http.Client{Transport: bearerTransport{tokenAlice}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "slow"})
		if err != nil {
			t.Fatal(err)
		}
		return res.Content[0].(*mcp.TextContent).Text
	}
	if got := run(nil); got != "no-elicitation" {
		t.Errorf("client without capability: %q", got)
	}
	opts := &mcp.ClientOptions{ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"approve": true}}, nil
	}}
	if got := run(opts); got != "accept:true" {
		t.Errorf("client with elicitation: %q", got)
	}
}
