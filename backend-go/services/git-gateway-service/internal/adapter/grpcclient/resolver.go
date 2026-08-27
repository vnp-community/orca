// Package grpcclient contains git-gateway-service's outbound gRPC client
// adapters for infra-fleet-service — this service's dependency for
// connection resolution and relay dispatch (git-gateway-service.md §7: "the
// only two Go services that talk to the execution plane"). Both adapters
// here (ConnectionResolver in this file, RelayExecutor in
// relay_executor.go) wrap the same *grpc.ClientConn dialed once in
// cmd/server/main.go and forward the caller's tenant identity on every RPC
// via withTenantMetadata (tenant_forwarding.go).
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// ConnectionResolver implements usecase.ConnectionResolver by calling
// infra-fleet-service's ResolveConnection RPC.
type ConnectionResolver struct {
	client infrafleetv1.InfraFleetServiceClient
}

// Dial opens a gRPC client connection to infra-fleet-service at addr — the
// same lazy-dial pattern api-gateway's internal/adapter/grpc/dial.go uses
// (grpc.NewClient doesn't block on connect, so infra-fleet-service being
// down doesn't fail this service's startup). Shared by both
// NewConnectionResolver and NewRelayExecutor's callers in cmd/server/main.go
// so a single connection backs both adapters.
//
// Insecure transport credentials — acceptable for local dev only; see
// api-gateway's Dial doc comment for the production mTLS gap this mirrors.
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return conn, nil
}

// NewConnectionResolver wraps an already-constructed infrafleetv1 client —
// used with Dial's connection in cmd/server/main.go, and with a fake client
// in tests.
func NewConnectionResolver(client infrafleetv1.InfraFleetServiceClient) *ConnectionResolver {
	return &ConnectionResolver{client: client}
}

// ResolveConnection asks infra-fleet-service which host owns worktreeID.
//
// git-gateway-service's worktreeID IS the infra-fleet-service connectionId
// (git-gateway-service.md §7's dependency diagram; ResolveConnectionRequest
// has no separate worktree field, only connection_id) — so it is passed
// through verbatim as ConnectionId, and echoed back as ResolvedConnection's
// ConnectionID on a successful resolve rather than re-derived from
// resp.GetDevServer().GetId() (DevServer's id identifies the *host*, not the
// connection/worktree — see ResolvedConnection's doc comment in
// usecase/ports.go: "Connected=true, ConnectionID populated" describes the
// connection being resolved, which is worktreeID itself).
func (r *ConnectionResolver) ResolveConnection(ctx context.Context, worktreeID string) (usecase.ResolvedConnection, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.ResolvedConnection{}, err
	}

	resp, err := r.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{
		ConnectionId: worktreeID,
	})
	if err != nil {
		return usecase.ResolvedConnection{}, fmt.Errorf("grpcclient: ResolveConnection(%q): %w", worktreeID, err)
	}

	if !resp.GetConnected() {
		return usecase.ResolvedConnection{Connected: false, RepoPath: worktreeID}, nil
	}
	return usecase.ResolvedConnection{
		Connected:    true,
		ConnectionID: worktreeID,
		RepoPath:     resp.GetRepoPath(),
		Mode:         resp.GetDevServer().GetMode(),
	}, nil
}
