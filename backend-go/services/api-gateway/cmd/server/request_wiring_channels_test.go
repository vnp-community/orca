package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// fakeRequestServer plays request-service over a real gRPC connection: it records the
// metadata of each call, enforces the internal token with both Guard instances the real
// service uses, and leaves every RPC it does not implement Unimplemented.
type fakeRequestServer struct {
	requestv1.UnimplementedRequestServiceServer
	requestv1.UnimplementedApprovalServiceServer

	mu   sync.Mutex
	seen []metadata.MD
}

func (s *fakeRequestServer) note(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	s.seen = append(s.seen, md)
	s.mu.Unlock()
}

func (s *fakeRequestServer) GetRequest(ctx context.Context, in *requestv1.GetRequestRequest) (*requestv1.GetRequestResponse, error) {
	s.note(ctx)
	if in.GetId() == "missing" {
		return nil, status.Error(codes.NotFound, "REQUEST_NOT_FOUND: no such request")
	}
	return &requestv1.GetRequestResponse{Request: &requestv1.Request{Id: in.GetId(), Title: "T", Body: "B", Status: "new"}}, nil
}

func (s *fakeRequestServer) ChangeRequestType(ctx context.Context, _ *requestv1.ChangeRequestTypeRequest) (*requestv1.ChangeRequestTypeResponse, error) {
	s.note(ctx)
	return nil, status.Error(codes.FailedPrecondition, "REQUEST_VERSION_CONFLICT: reload and retry")
}

func startFakeRequestServer(t *testing.T, token string) (*fakeRequestServer, []grpc.DialOption) {
	t.Helper()
	fake := &fakeRequestServer{}
	methods := []string{
		requestv1.RequestService_GetRequest_FullMethodName, requestv1.RequestService_ChangeRequestType_FullMethodName,
		requestv1.ApprovalService_GetApproval_FullMethodName,
	}
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(internalcaller.Guard(token, methods...)),
		grpc.ChainStreamInterceptor(internalcaller.StreamGuard(token, methods...)),
	)
	requestv1.RegisterRequestServiceServer(srv, fake)
	requestv1.RegisterApprovalServiceServer(srv, fake)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return fake, []grpc.DialOption{grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() })}
}

func requestChannelRegistry(t *testing.T, dial []grpc.DialOption) *wscompat.Registry {
	t.Helper()
	rc, ac, conn, err := buildRequestClients("passthrough:///request-service", slog.New(slog.NewTextHandler(io.Discard, nil)), dial...)
	if err != nil || conn == nil {
		t.Fatalf("buildRequestClients: conn=%v err=%v", conn, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	reg := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(reg, wscompat.ChannelDeps{Request: rc, Approval: ac})
	return reg
}

func dispatchJSON(t *testing.T, reg *wscompat.Registry, ctx context.Context, channel, args string) (map[string]any, error) {
	t.Helper()
	res, err := reg.Dispatch(ctx, wscompat.Identity{TenantID: "t1", UserID: "u1", Role: "user"}, channel, []json.RawMessage{json.RawMessage(args)})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(res)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m, nil
}

func TestBuildRequestClients_EmptyAddressIsNotDialed(t *testing.T) {
	rc, ac, conn, err := buildRequestClients("", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if rc != nil || ac != nil || conn != nil || err != nil {
		t.Fatalf("empty address must give nil clients, got %v %v %v %v", rc, ac, conn, err)
	}
}

// The full path of a WS call: channel -> client -> gRPC with token, identity and actor type -> server.
func TestRequestChannels_OverGRPC_TokenIdentityAndActor(t *testing.T) {
	t.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", "s3cret")
	fake, dial := startFakeRequestServer(t, "s3cret")
	reg := requestChannelRegistry(t, dial)

	res, err := dispatchJSON(t, reg, tenant.WithActorType(context.Background(), tenant.ActorAgent), "request.get", `{"id":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if rq := res["request"].(map[string]any); rq["id"] != "r1" || rq["body"] != "B" || rq["status"] != "new" {
		t.Errorf("result = %v", res)
	}
	md := fake.seen[0]
	for key, want := range map[string]string{"x-orca-tenant-id": "t1", "x-orca-user-id": "u1", "x-orca-actor-type": "agent", internalcaller.MetadataKey: "s3cret"} {
		if got := md.Get(key); len(got) == 0 || got[0] != want {
			t.Errorf("metadata %s = %v, want %s", key, got, want)
		}
	}
	if got := md.Get("x-orca-role"); len(got) == 0 || got[0] != "user" {
		t.Errorf("role metadata = %v", got)
	}
}

func TestRequestChannels_OverGRPC_WrongOrMissingTokenIsRefused(t *testing.T) {
	for _, tok := range []string{"", "wrong"} {
		t.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", tok)
		fake, dial := startFakeRequestServer(t, "s3cret")
		reg := requestChannelRegistry(t, dial)
		_, err := dispatchJSON(t, reg, context.Background(), "request.get", `{"id":"r1"}`)
		if err == nil || !strings.HasPrefix(err.Error(), "REQUEST_FORBIDDEN:") {
			t.Errorf("token %q: err = %v, want REQUEST_FORBIDDEN", tok, err)
		}
		if len(fake.seen) != 0 {
			t.Errorf("token %q reached the handler", tok)
		}
	}
}

func TestRequestChannels_OverGRPC_ErrorsReachTheClientClearly(t *testing.T) {
	t.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", "s3cret")
	_, dial := startFakeRequestServer(t, "s3cret")
	reg := requestChannelRegistry(t, dial)
	cases := []struct{ channel, args, want string }{
		{"request.get", `{"id":"missing"}`, "REQUEST_NOT_FOUND: no such request"},
		{"request.changeType", `{"id":"r1","toType":"bug","reason":"x","expectedVersion":1}`, "REQUEST_VERSION_CONFLICT: reload and retry"},
		// RPCs another agent has not implemented yet answer Unimplemented, surfaced as a clear code.
		{"request.list", `{}`, "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
		{"solution.list", `{"requestId":"r1"}`, "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
		{"backlog.requests", `{}`, "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
		{"approval.get", `{"id":"a1"}`, "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
		{"approval.approve", `{"id":"a1","expectedVersion":1,"expectedDigest":"d"}`, "REQUEST_NOT_IMPLEMENTED: this operation is not available on the server yet"},
	}
	for _, c := range cases {
		_, err := dispatchJSON(t, reg, context.Background(), c.channel, c.args)
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: err = %v, want %q", c.channel, err, c.want)
		}
	}
}
