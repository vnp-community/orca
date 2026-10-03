package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type listAgentsClient struct {
	infrafleetv1.InfraFleetServiceClient
	req  *infrafleetv1.ListAgentSessionsRequest
	md   metadata.MD
	resp *infrafleetv1.ListAgentSessionsResponse
	err  error
}

func (c *listAgentsClient) ListAgentSessions(ctx context.Context, in *infrafleetv1.ListAgentSessionsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListAgentSessionsResponse, error) {
	c.req = in
	c.md, _ = metadata.FromOutgoingContext(ctx)
	return c.resp, c.err
}

func TestInfraAgentLister_QueriesByMcpOriginAsTheSessionOwner(t *testing.T) {
	c := &listAgentsClient{resp: &infrafleetv1.ListAgentSessionsResponse{Sessions: []*infrafleetv1.AgentSession{{Id: "a1", Status: "running"}}}}
	got, err := infraAgentLister{c: c}.ListMcpAgentSessions(context.Background(), wscompat.Identity{TenantID: "t1", UserID: "u1"}, "sess-9")
	if err != nil || len(got) != 1 || got[0].SessionID != "a1" || got[0].Status != "running" {
		t.Fatalf("got %+v err %v", got, err)
	}
	if c.req.GetOriginType() != "mcp" || c.req.GetOriginSessionId() != "sess-9" || !c.req.GetActiveOnly() {
		t.Fatalf("request = %+v", c.req)
	}
	if v := c.md.Get(grpcmw.MetadataTenantID); len(v) != 1 || v[0] != "t1" {
		t.Fatalf("tenant metadata = %v", c.md)
	}
	if v := c.md.Get(grpcmw.MetadataUserID); len(v) != 1 || v[0] != "u1" {
		t.Fatalf("user metadata = %v", c.md)
	}
}

func TestInfraAgentLister_OldInfraFleetDegradesAndOtherErrorsSurface(t *testing.T) {
	c := &listAgentsClient{err: status.Error(codes.Unimplemented, "nope")}
	if got, err := (infraAgentLister{c: c}).ListMcpAgentSessions(context.Background(), wscompat.Identity{TenantID: "t"}, "s"); err != nil || got != nil {
		t.Fatalf("Unimplemented must degrade quietly: %v %v", got, err)
	}
	c.err = status.Error(codes.Unavailable, "down")
	if _, err := (infraAgentLister{c: c}).ListMcpAgentSessions(context.Background(), wscompat.Identity{TenantID: "t"}, "s"); err == nil {
		t.Fatal("transport errors must surface so the reaper event is redelivered")
	}
}

type recordingPublisher struct {
	subject string
	ev      commoneventbus.Event
	err     error
}

func (r *recordingPublisher) Publish(_ context.Context, subject string, ev commoneventbus.Event) error {
	r.subject, r.ev = subject, ev
	return r.err
}

func TestIdleStoppedNotifier_PublishesTheMcpSubjectAndSurvivesFailures(t *testing.T) {
	pub := &recordingPublisher{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	idleStoppedNotifier(pub, log)("t1", "u1", "sess-1", "pty-7")
	if pub.subject != "orca.mcp.terminal.idlestopped" || pub.ev.TenantID != "t1" || pub.ev.ID != "idlestopped:pty-7" {
		t.Fatalf("published %q %+v", pub.subject, pub.ev)
	}
	var p map[string]string
	if err := json.Unmarshal(pub.ev.Payload, &p); err != nil || p["session_id"] != "sess-1" || p["pty_id"] != "pty-7" || p["user_id"] != "u1" || p["reason"] != "idle" {
		t.Fatalf("payload = %v (%v)", p, err)
	}
	pub.err = errors.New("nats down")
	idleStoppedNotifier(pub, log)("t1", "u1", "sess-1", "pty-8") // must not panic
}
