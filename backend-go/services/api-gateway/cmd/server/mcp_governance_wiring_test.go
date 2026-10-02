package main

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcppolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type tokenRecorder struct {
	mcpv1.UnimplementedMcpServiceServer
	token chan string
}

func (r *tokenRecorder) GetKillState(ctx context.Context, _ *mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	v := md.Get(internalcaller.MetadataKey)
	if len(v) > 0 {
		r.token <- v[0]
	} else {
		r.token <- ""
	}
	return &mcpv1.GetKillStateResponse{}, nil
}

func TestDialMCPServicePresentsTheInternalCallerToken(t *testing.T) {
	t.Setenv("MCP_INTERNAL_CALLER_TOKEN", "s3cret")
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rec := &tokenRecorder{token: make(chan string, 1)}
	srv := grpc.NewServer()
	mcpv1.RegisterMcpServiceServer(srv, rec)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	conn, err := dialMCPService(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := mcpv1.NewMcpServiceClient(conn).GetKillState(ctx, &mcpv1.GetKillStateRequest{}); err != nil {
		t.Fatal(err)
	}
	if got := <-rec.token; got != "s3cret" {
		t.Fatalf("token on the wire = %q", got)
	}
}

func TestBuildMCPGovernanceDegradesToFailClosedWithoutClient(t *testing.T) {
	gate, guard, err := buildMCPGovernance(nil, slog.Default())
	if err != nil || gate != nil || guard != nil {
		t.Fatalf("no client must yield a nil gate (=> FailClosedGate) and no guard: %v %v %v", gate, guard, err)
	}
	var v mcpserver.TokenVerifier = nil
	if withKillGuard(v, nil) != nil {
		t.Fatal("no guard leaves the verifier untouched")
	}
	t.Setenv("MCP_APPROVAL_MAX_WAIT", "soon")
	if _, _, err := buildMCPGovernance(mcpv1.NewMcpServiceClient(nil), slog.Default()); err == nil {
		t.Fatal("bad duration must fail startup, not silently default")
	}
	t.Setenv("MCP_APPROVAL_MAX_WAIT", "40s")
	gate, guard, err = buildMCPGovernance(mcpv1.NewMcpServiceClient(nil), slog.Default())
	if err != nil || gate == nil || guard == nil {
		t.Fatal(err)
	}
	if _, ok := gate.(*mcppolicy.Gate); !ok {
		t.Fatal("real gate expected")
	}
}
