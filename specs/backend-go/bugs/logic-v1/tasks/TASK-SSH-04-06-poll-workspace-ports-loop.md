# TASK-SSH-04-06: `PollWorkspacePorts` — periodic scan/diff/tunnel/notify loop (BR-SSH-15/18)

**From Solution:** SOL-SSH-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/poll_workspace_ports.go` (new)
**Depends on:** TASK-SSH-04-01, TASK-SSH-04-03, TASK-SSH-04-04, TASK-SSH-04-05
**Status:** `[ ]` TODO

---

## Context

This is the actual auto-port-forwarding feature: a 2s-interval loop per
established relay-ssh `Connection` that scans (`ScanWorkspacePorts`, now
correctly relaying to `ports.detect` per TASK-SSH-04-01), diffs against
known open tunnels, opens a `sshconn.Tunnel` (TASK-SSH-04-05) on a newly
allocated local port (TASK-SSH-04-04) for each newly-seen remote port, and
tears down tunnels for ports that stop appearing (BR-SSH-18).

## Changes to make

Add a narrow publish port to
`backend-go/services/infra-fleet-service/internal/usecase/ports.go`:

```go
// PortForwardEventPublisher publishes port-forward lifecycle events for
// TASK-SSH-04-08's push path to consume. Defined here (consumer-side) per
// this codebase's Dependency Inversion convention.
type PortForwardEventPublisher interface {
	Publish(ctx context.Context, event string, pf domain.PortForward)
}

// TunnelOpener narrows sshconn.Connection.Forward to what this package needs.
type TunnelOpener interface {
	Forward(localPort, remotePort int) (Tunnel, error)
}

// Tunnel narrows sshconn.Tunnel to its Close method — the only thing
// PollWorkspacePorts calls on it directly.
type Tunnel interface {
	Close() error
}
```

Create `backend-go/services/infra-fleet-service/internal/usecase/poll_workspace_ports.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// PollWorkspacePorts runs one 2s-interval loop PER established relay-ssh
// Connection (BL-SSH-04's "scan every 2 seconds") — started by
// EstablishConnection on success, torn down on ctx cancellation
// (TeardownConnection/worktree deletion). Per-connection goroutines, not a
// shared scheduler: port scanning is much higher-frequency and inherently
// per-connection than infra-fleet-service.md §8's fleet-HEALTH leader
// election, which this deliberately does not reuse.
type PollWorkspacePorts struct {
	scan   *ScanWorkspacePorts
	alloc  PortAllocator
	tunnel TunnelOpener
	repo   PortForwardRepository
	events PortForwardEventPublisher
}

// PortAllocator narrows portalloc.Allocator to what this package needs.
type PortAllocator interface {
	Allocate(portForwardID string) (int, error)
	Release(localPort int)
}

func NewPollWorkspacePorts(scan *ScanWorkspacePorts, alloc PortAllocator, tunnel TunnelOpener, repo PortForwardRepository, events PortForwardEventPublisher) *PollWorkspacePorts {
	return &PollWorkspacePorts{scan: scan, alloc: alloc, tunnel: tunnel, repo: repo, events: events}
}

// Run blocks until ctx is cancelled, tearing down every open tunnel on exit.
func (p *PollWorkspacePorts) Run(ctx context.Context, tenantID, connectionID, worktreeID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	known := map[int32]trackedForward{} // remotePort -> {domain.PortForward, sshconn.Tunnel}
	defer func() {
		for _, tf := range known {
			p.teardown(tenantID, tf)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			detected, err := p.scan.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: connectionID, WorktreeID: worktreeID})
			if err != nil {
				continue // transient scan failure — try again next tick, don't tear down existing tunnels on one bad poll
			}

			seen := make(map[int32]bool, len(detected))
			for _, d := range detected {
				seen[d.Port] = true
				if _, exists := known[d.Port]; exists {
					continue
				}
				tf, err := p.openForward(ctx, tenantID, connectionID, d)
				if err != nil {
					continue // couldn't allocate/tunnel this one — retried next tick
				}
				known[d.Port] = tf
				p.events.Publish(ctx, "dev_server.port_opened", tf.forward)
			}

			for remotePort, tf := range known {
				if !seen[remotePort] { // BR-SSH-18: remote port closed
					p.teardown(tenantID, tf)
					delete(known, remotePort)
					p.events.Publish(ctx, "dev_server.port_closed", tf.forward)
				}
			}
		}
	}
}

type trackedForward struct {
	forward domain.PortForward
	tunnel  Tunnel
}

func (p *PollWorkspacePorts) openForward(ctx context.Context, tenantID, connectionID string, d DetectedPort) (trackedForward, error) {
	id := uuid.NewString()
	localPort, err := p.alloc.Allocate(id)
	if err != nil {
		return trackedForward{}, err
	}
	tun, err := p.tunnel.Forward(localPort, int(d.Port))
	if err != nil {
		p.alloc.Release(localPort)
		return trackedForward{}, err
	}
	pf := domain.PortForward{
		ID: id, TenantID: tenantID, ConnectionID: connectionID,
		LocalPort: localPort, RemotePort: int(d.Port),
		ProcessName: d.ProcessName, Status: domain.PortForwardStatusActive,
	}
	saved, err := p.repo.Create(ctx, pf)
	if err != nil {
		_ = tun.Close()
		p.alloc.Release(localPort)
		return trackedForward{}, err
	}
	return trackedForward{forward: saved, tunnel: tun}, nil
}

func (p *PollWorkspacePorts) teardown(tenantID string, tf trackedForward) {
	_ = tf.tunnel.Close()
	p.alloc.Release(tf.forward.LocalPort)
	_ = p.repo.UpdateStatus(context.Background(), tenantID, tf.forward.ID, domain.PortForwardStatusClosed)
}
```

Wire `Run` to start from `EstablishConnection`'s success path (spawn
`go pollWorkspacePorts.Run(connectionLifetimeCtx, ...)`, where
`connectionLifetimeCtx` is cancelled by `TeardownConnection` — reuses the
same `closeCh`-style cancellation `SOL-SSH-03`'s `TeardownConnection`
already introduces).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestPollWorkspacePorts -v
```

Expected new test (`poll_workspace_ports_test.go`, fake scanner/allocator/
tunnel/repo/publisher): a newly-seen port opens a tunnel + publishes
`port_opened`; a port that stops appearing tears its tunnel down +
publishes `port_closed`; a transient scan error leaves existing tunnels
untouched; `ctx` cancellation tears down every open tunnel.
