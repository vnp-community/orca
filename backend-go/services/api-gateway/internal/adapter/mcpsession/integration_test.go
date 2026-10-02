//go:build integration

package mcpsession_test

// Two gateway replicas, a REAL NATS server (testcontainers): the resume buffer
// is the JetStream MCPSSE stream and cancel/close travel over core NATS. Only
// the session table is an in-process stand-in for mcp-service's database.

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

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpsession"
)

type tool func(ctx context.Context) (*mcp.CallToolResult, error)

func (f tool) CallTool(ctx context.Context, _ mcpserver.Principal, _ string, _ json.RawMessage) (*mcp.CallToolResult, error) {
	return f(ctx)
}

type rep struct {
	h   *mcpserver.Handler
	srv *httptest.Server
	url string
}

var verifier = mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
	"tok-alice": {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}},
}}

func newRep(t *testing.T, natsURL string, store mcpserver.SessionStore, clk func() time.Time, exec mcpserver.ToolExecutor) *rep {
	t.Helper()
	ctx := context.Background()
	log, err := mcpsession.NewLog(ctx, natsURL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	eph, closeEph, err := eventbus.NewEphemeral(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeEph)
	srv := httptest.NewUnstartedServer(nil)
	base := "http://" + srv.Listener.Addr().String()
	h := mcpserver.NewHandler(mcpserver.Deps{
		Config: mcpserver.Config{ResourceURL: base + "/mcp", IssuerURL: "https://a.example", MaxBodyBytes: 1 << 16, SessionIdleTTL: time.Minute},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Verifier: verifier, Executor: exec, CursorKeys: [][]byte{[]byte("k-0123456789abcdef0123456789abcd")},
		Sessions: store, EventStore: mcpsession.NewJetStreamEventStore(log, 0), Signals: eph, Now: clk,
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv.Config.Handler = r
	srv.Start()
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close(); h.Close() })
	return &rep{h: h, srv: srv, url: base + "/mcp"}
}

func (r *rep) do(ctx context.Context, method, sid, body string, hdr map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer tok-alice")
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

func (r *rep) mustDo(t *testing.T, ctx context.Context, method, sid, body string, hdr map[string]string) *http.Response {
	t.Helper()
	resp, err := r.do(ctx, method, sid, body, hdr)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (r *rep) session(t *testing.T) string {
	t.Helper()
	resp := r.mustDo(t, context.Background(), http.MethodPost, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1"}}}`, nil)
	sid := resp.Header.Get("Mcp-Session-Id")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	ack := r.mustDo(t, context.Background(), http.MethodPost, sid, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	_ = ack.Body.Close()
	if sid == "" || ack.StatusCode != http.StatusAccepted {
		t.Fatalf("session setup: sid=%q ack=%d", sid, ack.StatusCode)
	}
	return sid
}

type ev struct{ ID, Data string }

func readSSE(rd io.Reader, n int) []ev {
	var out []ev
	var cur ev
	br := bufio.NewReader(rd)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return out
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "id: "):
			cur.ID = line[4:]
		case strings.HasPrefix(line, "data: "):
			cur.Data = line[6:]
		case line == "":
			if cur.Data != "" {
				out = append(out, cur)
				if n > 0 && len(out) >= n {
					return out
				}
			}
			cur = ev{}
		}
	}
}

func idx(t *testing.T, id string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(id[strings.LastIndex(id, "_")+1:], "%d", &n); err != nil {
		t.Fatalf("bad id %q", id)
	}
	return n
}

func TestTwoReplicaResumeOverRealNATS(t *testing.T) {
	natsURL := testutil.StartNATS(t)
	var ns atomic.Int64
	ns.Store(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC).UnixNano())
	clk := func() time.Time { return time.Unix(0, ns.Load()).UTC() }
	store := mcpserver.NewMemorySessionStore(time.Minute, clk)
	release := make(chan struct{})
	exec := tool(func(ctx context.Context) (*mcp.CallToolResult, error) {
		for i := 0; i < 30; i++ {
			ns.Add(int64(250 * time.Millisecond))
			mcpserver.ReportProgress(ctx, float64(i), 30, "")
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil
	})
	a, b := newRep(t, natsURL, store, clk, exec), newRep(t, natsURL, store, clk, exec)
	sid := a.session(t)

	ctxA, cut := context.WithCancel(context.Background())
	resp := a.mustDo(t, ctxA, http.MethodPost, sid, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"slow","arguments":{},"_meta":{"progressToken":"p"}}}`, nil)
	first := readSSE(resp.Body, 12)
	cut()
	_ = resp.Body.Close()
	if len(first) != 12 || idx(t, first[11].ID) != 11 {
		t.Fatalf("first leg: %d events", len(first))
	}

	rctx, rcancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer rcancel()
	got := b.mustDo(t, rctx, http.MethodGet, sid, "", map[string]string{"Last-Event-ID": first[11].ID})
	defer got.Body.Close()
	if got.StatusCode != 200 {
		t.Fatalf("resume on B: %d", got.StatusCode)
	}
	rest := readSSE(got.Body, 18) // events 12..29
	for i, e := range rest {
		if idx(t, e.ID) != 12+i {
			t.Fatalf("event %d has id %s, want ordinal %d", i, e.ID, 12+i)
		}
	}
	if len(rest) != 18 {
		t.Fatalf("replayed %d events, want 18", len(rest))
	}
	close(release)
	tail := readSSE(got.Body, 0)
	if len(tail) != 1 || idx(t, tail[0].ID) != 30 || !strings.Contains(tail[0].Data, `"id":7`) {
		t.Fatalf("tail: %+v", tail)
	}
}

func TestDeleteOnOtherReplicaCancelsToolOverRealNATS(t *testing.T) {
	natsURL := testutil.StartNATS(t)
	store := mcpserver.NewMemorySessionStore(time.Minute, nil)
	running, cancelled := make(chan struct{}, 1), make(chan struct{}, 1)
	exec := tool(func(ctx context.Context) (*mcp.CallToolResult, error) {
		running <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	})
	a, b := newRep(t, natsURL, store, time.Now, exec), newRep(t, natsURL, store, time.Now, exec)
	sid := a.session(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if resp, err := a.do(ctx, http.MethodPost, sid, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"slow","arguments":{}}}`, nil); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()
	<-running
	// B has never seen the session: it adopts it from the store to serve DELETE,
	// closes it and signals A over core NATS.
	del := b.mustDo(t, context.Background(), http.MethodDelete, sid, "", nil)
	_ = del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE: %d", del.StatusCode)
	}
	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("the tool on replica A was not cancelled by DELETE on replica B")
	}
}
