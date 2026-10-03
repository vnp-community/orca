package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
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

func counterValue(t *testing.T, m *mcpmetrics.Metrics, result string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "orca_mcp_terminal_idle_stop_events_total" {
			continue
		}
		for _, mm := range mf.GetMetric() {
			if mm.GetLabel()[0].GetValue() == result {
				return mm.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func TestIdleCloseObserver_CountsCloseResultAndToleratesNilMetrics(t *testing.T) {
	m := mcpmetrics.New()
	obs := idleCloseObserver(m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	obs("t1", "u1", "sess-1", "pty-1", "Cursor", nil)
	obs("t1", "u1", "sess-1", "pty-2", "Cursor", errors.New("infra-fleet down"))
	obs("t1", "u1", "sess-1", "pty-3", "Cursor", nil)
	if counterValue(t, m, "closed") != 2 || counterValue(t, m, "failed") != 1 {
		t.Fatalf("closed=%v failed=%v", counterValue(t, m, "closed"), counterValue(t, m, "failed"))
	}
	idleCloseObserver(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))("t", "u", "s", "p", "", errors.New("x"))
}
