// Package usecase holds infra-fleet-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// DevServerAgentClient is defined here rather than in adapter/devserveragent
// deliberately — per
// specs/backend-go/services/infra-fleet-service.md §6 ("usecase/ ... defines
// DevServerAgentClient port here (not adapter/)"), the wire-protocol client
// is an outbound adapter like any other, and the usecase layer must not
// depend on its concrete package.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerRepository is the persistence port for the dev server registry.
// Implemented by internal/adapter/postgres against this service's own
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type DevServerRepository interface {
	Register(ctx context.Context, devServer domain.DevServer) (domain.DevServer, error)
	Get(ctx context.Context, tenantID, id string) (domain.DevServer, error)
	List(ctx context.Context, tenantID string) ([]domain.DevServer, error)
}

// SshTargetRepository is the persistence port for SSH target registration.
type SshTargetRepository interface {
	Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
	// Get fetches an SSH target scoped to tenantID — same tenant-join
	// requirement as DevServerRepository.Get, see
	// specs/backend-go/services/infra-fleet-service.md §9.
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
}

// SshTargetResolver is the narrow read port adapter/devserveragent.Client
// needs to resolve a DevServer.SSHTargetID into a full domain.SshTarget
// before dialing via sshconn.Connector for relay-ssh mode. Same method as
// SshTargetRepository.Get, declared as its own interface rather than reused
// directly — this codebase's convention is that a port is defined where it
// is consumed (see this file's own doc comment), and devserveragent is a
// different consumer than the usecases that hold SshTargetRepository.
// Implemented by postgres.SshTargetStore, same as SshTargetRepository.
type SshTargetResolver interface {
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
}

// ConnectionRepository is the persistence port for the write side of
// infra.connections (migrations/0002_connections) — the real routing model
// that replaced the connectionId==dev_server.id equation. Kept separate from
// ConnectionResolver (the read side) the same way DevServerRepository and
// ConnectionResolver already are two narrow ports over one Repository.
type ConnectionRepository interface {
	CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error)
}

// ConnectionResolver is THE core coordination/execution dispatch primitive
// of this service — see
// specs/backend-go/services/infra-fleet-service.md §7's sequence diagram.
// Every "does this worktree/session have a connectionId" branch in the
// system reduces to a call through this port: found means relay to that
// DevServer, not-found means the caller executes locally.
//
// tenantID is threaded through explicitly (mirroring usage-service's
// Repository port convention) rather than pulled from ctx inside this
// interface's implementations, even though callers extract it from ctx via
// common/tenant first — see specs/backend-go/services/infra-fleet-service.md
// §9's "ResolveConnection must join through tenant_id on every lookup"
// requirement: an explicit parameter makes that join impossible to forget
// at any implementation's call site, and keeps the port trivially fakeable
// in tests without a context-plumbing helper.
type ConnectionResolver interface {
	// ResolveConnection looks up connectionID within tenantID's scope.
	// connected=false with a nil error means "no dev server owns this
	// connectionId" — the caller's cue to execute locally, not an error
	// condition. conn carries the per-connection metadata (RepoPath,
	// WorktreeID) callers like git-gateway-service's RelayExecutor need
	// alongside devServer — zero-value when connected is false.
	ResolveConnection(ctx context.Context, tenantID, connectionID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)
}

// FleetHealthPort is the read port over fleet health samples. The
// health-polling writer side (the 30s-cadence poller from
// specs/backend-go/services/infra-fleet-service.md §8) is not implemented in
// this scaffold — see this service's README "Known gaps".
type FleetHealthPort interface {
	GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error)
}

// DevServerAgentClient is the port to the Dev Server Agent execution plane —
// implemented by adapter/devserveragent against the EXISTING TS wire
// protocol (Option A, see
// specs/backend-go/architecture/08-inter-service-communication.md), NOT a
// new gRPC contract. Real for relay-websocket mode (Epic A, 2026-08-17);
// direct-websocket/relay-ssh still return ErrConnectionModeNotImplemented.
// See adapter/devserveragent's package doc comment and this service's
// README "Known gaps" for exactly what's still missing.
type DevServerAgentClient interface {
	// Exec dispatches one JSON-RPC method call (e.g. "ports.scan",
	// "pty.spawn", "preflight.check") to the agent over devServer's resolved
	// transport mode and returns its decoded result.
	Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)
	// Health performs an agent-level reachability/handshake check, distinct
	// from the SSH-exec-based fleet health poll that GetFleetHealth reads.
	Health(ctx context.Context, devServer domain.DevServer) (bool, error)
}
