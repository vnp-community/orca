package grpcclient

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func ctxWithIdentity() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111"), "22222222-2222-2222-2222-222222222222")
}

// fakeInfra embeds the generated interface so only the calls under test need bodies.
type fakeInfra struct {
	infrafleetv1.InfraFleetServiceClient
	mu           sync.Mutex
	connected    bool
	repoPath     string
	worktreeID   string
	health       []*infrafleetv1.DevServerHealth
	replies      []string // content returned by successive ai.complete calls
	relayDelay   time.Duration
	relayCalls   []*infrafleetv1.RelayRequest
	devCalls     []*infrafleetv1.RelayByDevServerRequest
	lastMetadata metadata.MD
}

func (f *fakeInfra) ResolveConnection(ctx context.Context, _ *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return &infrafleetv1.ResolveConnectionResponse{Connected: f.connected, RepoPath: f.repoPath, WorktreeId: f.worktreeID}, nil
}

func (f *fakeInfra) GetFleetHealth(context.Context, *infrafleetv1.GetFleetHealthRequest, ...grpc.CallOption) (*infrafleetv1.GetFleetHealthResponse, error) {
	return &infrafleetv1.GetFleetHealthResponse{Statuses: f.health}, nil
}

func (f *fakeInfra) next() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.relayCalls) + len(f.devCalls)
	if n > len(f.replies) {
		n = len(f.replies)
	}
	if n == 0 {
		return ""
	}
	return f.replies[n-1]
}

func (f *fakeInfra) reply(ctx context.Context) (*infrafleetv1.RelayResponse, error) {
	if f.relayDelay > 0 {
		select {
		case <-time.After(f.relayDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b, _ := json.Marshal(map[string]any{"content": f.next()})
	return &infrafleetv1.RelayResponse{ResultJson: string(b)}, nil
}

func (f *fakeInfra) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.mu.Lock()
	f.relayCalls = append(f.relayCalls, in)
	f.lastMetadata, _ = metadata.FromOutgoingContext(ctx)
	f.mu.Unlock()
	return f.reply(ctx)
}

func (f *fakeInfra) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.mu.Lock()
	f.devCalls = append(f.devCalls, in)
	f.lastMetadata, _ = metadata.FromOutgoingContext(ctx)
	f.mu.Unlock()
	return f.reply(ctx)
}

type fakeProjects struct {
	projectv1.ProjectServiceClient
	repos    []*projectv1.Repo
	metadata metadata.MD
}

func (f *fakeProjects) ListRepos(ctx context.Context, _ *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	f.metadata, _ = metadata.FromOutgoingContext(ctx)
	return &projectv1.ListReposResponse{Repos: f.repos}, nil
}

const validReply = `{"type":"bug","size":"M","urgency":"normal","confidence":0.8,"reason":"stack trace"}`

var _ = grpcmw.MetadataTenantID

func tenantOnly() context.Context {
	return tenant.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
}
