package mcpserver_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

type closedLog struct {
	mu   sync.Mutex
	rows []string
}

func (l *closedLog) hook(row, reason string) {
	l.mu.Lock()
	l.rows = append(l.rows, row+":"+reason)
	l.mu.Unlock()
}
func (l *closedLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.rows...)
}

// OnSessionClosed fires for sessions that end for good (DELETE here, on the
// replica holding the connection and on the other replica via the signal), so
// terminals and agents the session started can be stopped (BE-MCP-SOL-009).
func TestOnSessionClosedHookFiresOnDeleteAcrossReplicas(t *testing.T) {
	cl := newCluster(t, 0)
	var la, lb closedLog
	a := cl.replica(t, nil, func(d *mcpserver.Deps) { d.OnSessionClosed = la.hook })
	b := cl.replica(t, nil, func(d *mcpserver.Deps) { d.OnSessionClosed = lb.hook })
	defer a.close()
	defer b.close()

	sid := a.initSession(t, tokenAlice)
	r := b.req(t, context.Background(), http.MethodPost, tokenAlice, sid, `{"jsonrpc":"2.0","id":2,"method":"ping"}`, nil) // adopt on b
	_ = r.Body.Close()
	del := a.req(t, context.Background(), http.MethodDelete, tokenAlice, sid, "", nil)
	_ = del.Body.Close()
	cl.bus.Wait()
	if got := la.snapshot(); len(got) != 1 {
		t.Fatalf("replica a hook calls %v", got)
	}
	if got := lb.snapshot(); len(got) != 1 {
		t.Fatalf("replica b (signal) hook calls %v", got)
	}
}

func TestOnSessionClosedHookFiresOnStoreExpiryButNotOnLocalEviction(t *testing.T) {
	cl := newCluster(t, 0)
	var l closedLog
	a := cl.replica(t, nil, func(d *mcpserver.Deps) { d.OnSessionClosed = l.hook })
	defer a.close()
	sid := a.initSession(t, tokenAlice)
	ping := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	cl.clk.Advance(30 * time.Second)
	r := a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, ping, nil)
	_ = r.Body.Close()
	if got := l.snapshot(); len(got) != 0 {
		t.Fatalf("live session reported closed: %v", got)
	}
	cl.clk.Advance(61 * time.Second)
	r = a.req(t, context.Background(), http.MethodPost, tokenAlice, sid, ping, nil)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("expired session answered %d", r.StatusCode)
	}
	if got := l.snapshot(); len(got) != 1 {
		t.Fatalf("expiry must fire the hook once, got %v", got)
	}
}
