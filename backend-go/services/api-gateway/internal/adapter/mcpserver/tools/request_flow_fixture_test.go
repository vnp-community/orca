package tools

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// fakeRequestService is a request-service double: embedded nil interfaces make any
// RPC the tests did not script panic, so an unexpected call cannot pass silently.
type fakeRequestService struct {
	requestv1.RequestServiceClient
	requestv1.ApprovalServiceClient

	mu       sync.Mutex
	creates  []*requestv1.CreateRequestRequest
	spawns   []*requestv1.SpawnChildRequestRequest
	createFn func(*requestv1.CreateRequestRequest) (*requestv1.CreateRequestResponse, error)
	getResp  *requestv1.Request
	solResp  *requestv1.ListSolutionsResponse
}

func (f *fakeRequestService) CreateRequest(_ context.Context, in *requestv1.CreateRequestRequest, _ ...grpc.CallOption) (*requestv1.CreateRequestResponse, error) {
	f.mu.Lock()
	f.creates = append(f.creates, in)
	f.mu.Unlock()
	if f.createFn != nil {
		return f.createFn(in)
	}
	return &requestv1.CreateRequestResponse{Request: &requestv1.Request{Id: "r1", Title: in.GetTitle(), SourceProvider: in.GetSource().GetProvider()}, Created: true}, nil
}

func (f *fakeRequestService) SpawnChildRequest(_ context.Context, in *requestv1.SpawnChildRequestRequest, _ ...grpc.CallOption) (*requestv1.SpawnChildRequestResponse, error) {
	f.mu.Lock()
	f.spawns = append(f.spawns, in)
	f.mu.Unlock()
	return &requestv1.SpawnChildRequestResponse{Child: &requestv1.Request{Id: "c1"}, Created: true}, nil
}

func (f *fakeRequestService) GetRequest(_ context.Context, _ *requestv1.GetRequestRequest, _ ...grpc.CallOption) (*requestv1.GetRequestResponse, error) {
	return &requestv1.GetRequestResponse{Request: f.getResp}, nil
}

func (f *fakeRequestService) ListSolutions(_ context.Context, _ *requestv1.ListSolutionsRequest, _ ...grpc.CallOption) (*requestv1.ListSolutionsResponse, error) {
	return f.solResp, nil
}

func (f *fakeRequestService) ListRequests(_ context.Context, _ *requestv1.ListRequestsRequest, _ ...grpc.CallOption) (*requestv1.ListRequestsResponse, error) {
	return &requestv1.ListRequestsResponse{Requests: []*requestv1.Request{{Id: "r1", Title: "t"}}, NextPageToken: "n"}, nil
}

// newRequestFlowExec builds an executor over the production channel registry with
// the fake request-service behind it, so tools run the real channel code.
func newRequestFlowExec(t *testing.T, fake *fakeRequestService, cfg Config) *Executor {
	t.Helper()
	reg := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(reg, wscompat.ChannelDeps{Request: fake, Approval: fake})
	cfg.Packs = allPacks
	gate := &mcpservertest.FakeGate{}
	cat, err := NewCatalog(AllSpecs(), cfg, gate, reg.Channels())
	if err != nil {
		t.Fatal(err)
	}
	return NewExecutor(cat, reg, gate, nil, cfg, quiet).WithGuards(Guards{
		SessionID:  func(context.Context) string { return "mcp-sess-1" },
		ClientName: func(context.Context) string { return "claude-code" },
	})
}
