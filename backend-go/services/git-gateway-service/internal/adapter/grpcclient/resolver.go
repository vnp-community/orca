// Package grpcclient contains stub outbound gRPC client adapters for
// infra-fleet-service — this service's dependency for connection resolution
// and relay dispatch (git-gateway-service.md §7: "the only two Go services
// that talk to the execution plane"). infra-fleet-service isn't running in
// this environment, so both adapters here are stubs, mirroring the
// cross-service-stub pattern other services use for dependencies that don't
// exist yet: ConnectionResolver always answers Connected=false (host-local),
// and RelayExecutor (relay_executor.go) always returns a clear
// "not implemented" error rather than silently pretending to succeed.
package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// ConnectionResolver is a STUB implementation of usecase.ConnectionResolver.
// It always reports Connected=false, so every request routes to the local
// git executor (internal/adapter/localgit) until infra-fleet-service's real
// ResolveConnection RPC is wired in.
//
// TODO(git-gateway-service): replace with a real gRPC client dialing
// infra-fleet-service (address from this service's config) and calling its
// ResolveConnection RPC, per git-gateway-service.md §7's dependency diagram.
type ConnectionResolver struct{}

func NewConnectionResolver() *ConnectionResolver {
	return &ConnectionResolver{}
}

func (r *ConnectionResolver) ResolveConnection(_ context.Context, worktreeID string) (usecase.ResolvedConnection, error) {
	// Stub: always host-local, and RepoPath stands in for project-service's
	// WorktreeResolver answer (see usecase/ports.go's ResolvedConnection doc
	// comment) by using worktreeID verbatim as the local path. That is only
	// safe for local/dev use of this scaffold — a real implementation must
	// resolve worktreeID -> filesystem path via project-service instead of
	// trusting a client-supplied path, per git-gateway-service.md §3's "never
	// trust a client-supplied host path" rule.
	return usecase.ResolvedConnection{
		Connected: false,
		RepoPath:  worktreeID,
	}, nil
}
