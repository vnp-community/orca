package mcpserver_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
)

const tokenCarol = "tok-carol" // tenant t2

var twoTenantVerifier = mcpservertest.StaticVerifier{Tokens: map[string]mcpserver.Principal{
	tokenAlice: {TenantID: "t1", UserID: "alice", Scopes: []string{"orca:read"}},
	tokenCarol: {TenantID: "t2", UserID: "carol", Scopes: []string{"orca:read"}},
}}

type changeCounter struct{ n atomic.Int32 }

func (c *changeCounter) connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "lc", Version: "1"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) { c.n.Add(1) },
	}).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: &http.Client{Transport: bearerTransport{token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestToolsListChanged_CapabilityOnlyWhenWired(t *testing.T) {
	off := newFixture(t, nil)
	cs := off.connect(t, tokenAlice)
	if c := cs.InitializeResult().Capabilities.Tools; c == nil || c.ListChanged {
		t.Fatalf("not wired: tools.listChanged must stay false, got %+v", c)
	}
	on := newFixture(t, func(d *mcpserver.Deps) { d.Config.ToolsListChanged = true })
	cs2 := on.connect(t, tokenAlice)
	if c := cs2.InitializeResult().Capabilities.Tools; c == nil || !c.ListChanged {
		t.Fatalf("wired: tools.listChanged must be advertised, got %+v", c)
	}
}

// Two replicas, the way production runs: the event reaches BOTH (each has its
// own ephemeral consumer) and each notifies only the sessions it holds, so the
// session on replica A hears it exactly once and another tenant hears nothing.
func TestToolsListChanged_EventOnEveryReplicaNotifiesOnlyThatTenantsSessionsOnce(t *testing.T) {
	c := newCluster(t, 100)
	mutate := func(d *mcpserver.Deps) {
		d.Config.ToolsListChanged = true
		d.Config.ToolsListChangedDebounce = 50 * time.Millisecond
		d.Verifier = twoTenantVerifier
	}
	a, b := c.replica(t, nil, mutate), c.replica(t, nil, mutate)

	var alice, carol changeCounter
	alice.connect(t, a.url, tokenAlice)
	carol.connect(t, b.url, tokenCarol)

	notifyAll := func(tenant string) {
		a.h.NotifyToolsChanged(tenant)
		b.h.NotifyToolsChanged(tenant)
	}
	// A burst of policy edits is one notification.
	for i := 0; i < 5; i++ {
		notifyAll("t1")
	}
	waitFor(t, "alice's list_changed", func() bool { return alice.n.Load() >= 1 })
	time.Sleep(300 * time.Millisecond)
	if alice.n.Load() != 1 {
		t.Fatalf("burst must be debounced to one notification, got %d", alice.n.Load())
	}
	if carol.n.Load() != 0 {
		t.Fatalf("another tenant's session was notified %d times", carol.n.Load())
	}

	notifyAll("t2")
	waitFor(t, "carol's list_changed", func() bool { return carol.n.Load() == 1 })
	time.Sleep(200 * time.Millisecond)
	if alice.n.Load() != 1 || carol.n.Load() != 1 {
		t.Fatalf("alice=%d carol=%d", alice.n.Load(), carol.n.Load())
	}

	// A later change notifies again (the debounce window is per burst, not forever).
	notifyAll("t1")
	waitFor(t, "second alice notification", func() bool { return alice.n.Load() == 2 })
}

func TestToolsListChanged_NotifyIsInertWhenNotWired(t *testing.T) {
	c := newCluster(t, 100)
	r := c.replica(t, nil, func(d *mcpserver.Deps) { d.Verifier = twoTenantVerifier })
	var alice changeCounter
	alice.connect(t, r.url, tokenAlice)
	r.h.NotifyToolsChanged("t1")
	time.Sleep(200 * time.Millisecond)
	if alice.n.Load() != 0 {
		t.Fatal("notifications must not be sent when the capability is not advertised")
	}
	(*mcpserver.Handler)(nil).NotifyToolsChanged("t1") // nil-safe
}
