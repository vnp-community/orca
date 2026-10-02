package wscompat

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type fakeEventStream struct {
	grpc.ClientStream
	events chan *mcpv1.McpEvent
}

func (s *fakeEventStream) Recv() (*mcpv1.McpEvent, error) {
	ev, ok := <-s.events
	if !ok {
		return nil, io.EOF
	}
	return ev, nil
}

type streamingClient struct {
	govClient
	s *fakeEventStream
}

func (c *streamingClient) StreamEvents(ctx context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (grpc.ServerStreamingClient[mcpv1.McpEvent], error) {
	c.rec(ctx)
	return c.s, nil
}

func TestMcpEventsSubscribe(t *testing.T) {
	s := &fakeEventStream{events: make(chan *mcpv1.McpEvent, 8)}
	c := &streamingClient{s: s}
	r := NewRegistry()
	RegisterMcpChannels(r, McpChannelDeps{Enabled: true, Client: c})
	h, ok := r.StreamHandlerFor("mcp.events.subscribe")
	if !ok {
		t.Fatal("stream channel not registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := h(ctx, Identity{TenantID: "t1", UserID: "u1", Role: "user"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.md.Get("x-orca-user-id")[0] != "u1" || c.md.Get("x-orca-tenant-id")[0] != "t1" {
		t.Fatalf("stream identity: %v", c.md)
	}
	s.events <- &mcpv1.McpEvent{Type: "approval.requested", Approval: sampleApproval()}
	s.events <- &mcpv1.McpEvent{Type: "approval.resolved", Id: "a1", Status: "approved"}
	s.events <- &mcpv1.McpEvent{Type: "killswitch.changed", Active: true, Reason: "incident"}
	s.events <- &mcpv1.McpEvent{Type: "from-the-future"}
	s.events <- &mcpv1.McpEvent{Type: "grant.revoked", GrantId: "g1"}
	var got []string
	for i := 0; i < 4; i++ {
		select {
		case ev := <-ch:
			if ev.Channel != "mcp.event" || len(ev.Args) != 1 {
				t.Fatalf("push shape: %+v", ev)
			}
			b, _ := json.Marshal(ev.Args[0])
			got = append(got, string(b))
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	}
	if !strings.Contains(got[0], `"type":"approval.requested"`) || !strings.Contains(got[0], `"paramsHash":"sha256:abc"`) ||
		got[1] != `{"id":"a1","status":"approved","type":"approval.resolved"}` || got[2] != `{"active":true,"reason":"incident","type":"killswitch.changed"}` ||
		got[3] != `{"grantId":"g1","type":"grant.revoked"}` {
		t.Fatalf("%v", got)
	}
	close(s.events)
	select {
	case _, open := <-ch:
		if open {
			t.Fatal("channel must close when the upstream stream ends so the client reconnects")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("not closed")
	}
}

func TestMcpEventsSubscribeGating(t *testing.T) {
	r := NewRegistry()
	RegisterMcpChannels(r, McpChannelDeps{Enabled: false, Client: &streamingClient{}})
	h, _ := r.StreamHandlerFor("mcp.events.subscribe")
	if _, err := h(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
		t.Fatalf("%v", err)
	}
	r = NewRegistry()
	RegisterMcpChannels(r, McpChannelDeps{Enabled: true, Client: &streamingClient{}})
	h, _ = r.StreamHandlerFor("mcp.events.subscribe")
	if _, err := h(context.Background(), Identity{TenantID: "t1"}, nil); err == nil {
		t.Fatal("no user, no stream")
	}
}
