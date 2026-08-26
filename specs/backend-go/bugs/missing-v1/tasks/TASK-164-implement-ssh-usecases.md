# TASK-164: Implement `ListSshTargets`/`GetSshState`/`EstablishConnection` usecases and wire them into `infra-fleet-service`'s gRPC server

**From Solution:** SOL-024
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `services/infra-fleet-service/internal/usecase/{list_ssh_targets,get_ssh_state,establish_connection}.go` (new), `services/infra-fleet-service/internal/adapter/grpc/server.go`, `services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-162, TASK-163
**Status:** `[x]` DONE — implemented in worktree `agent-a5714e047dcaed0fc`, **committed** as `56c5fbeff`. Build/vet/test clean. Pending merge.

---

## Dispatch model

- `ListSshTargets`, `GetSshState` — 🏠 always-local: Postgres reads
  against `infra-fleet-service`'s own tables, no Dev Server Agent hop.
- `EstablishConnection` — 🔌 always-remote: the connection-establishment
  act itself, via the Dev Server Agent relay protocol.

## Changes to make

### New file `internal/usecase/list_ssh_targets.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type ListSshTargets struct {
	repo SshTargetRepository
}

func NewListSshTargets(repo SshTargetRepository) *ListSshTargets {
	return &ListSshTargets{repo: repo}
}

func (uc *ListSshTargets) Execute(ctx context.Context) ([]domain.SshTarget, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	return uc.repo.List(ctx, tenantID)
}
```

### New file `internal/usecase/get_ssh_state.go`

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type SshStateInput struct {
	SshTargetID string
}

type SshState struct {
	Connected    bool
	ConnectionID string
	LastActivity *time.Time
}

// GetSshState is 🏠 always-local — reads whichever `connections` row (if
// any) currently binds this SSH target's dev server, never dials out.
// EstablishConnection (ssh.connect) is the only path that touches the
// network.
type GetSshState struct {
	sshTargets SshTargetRepository
	devServers DevServerRepository
	conns      ConnectionRepository
}

func NewGetSshState(sshTargets SshTargetRepository, devServers DevServerRepository, conns ConnectionRepository) *GetSshState {
	return &GetSshState{sshTargets: sshTargets, devServers: devServers, conns: conns}
}

func (uc *GetSshState) Execute(ctx context.Context, in SshStateInput) (SshState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return SshState{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	// No live dev server bound to this SSH target yet -> never connected.
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, in.SshTargetID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	conn, found, err := uc.conns.GetActiveByDevServer(ctx, tenantID, devServer.ID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	return SshState{Connected: true, ConnectionID: conn.ID, LastActivity: conn.LastActivityAt}, nil
}
```

### New file `internal/usecase/establish_connection.go`

```go
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// EstablishConnection performs the actual SSH + Dev Server Agent handshake
// synchronously — it is the connection-establishment act, not a record of
// one requested.
type EstablishConnection struct {
	sshTargets SshTargetRepository
	devServers DevServerRepository
	conns      ConnectionRepository
	agent      DevServerAgentClient
}

func NewEstablishConnection(sshTargets SshTargetRepository, devServers DevServerRepository, conns ConnectionRepository, agent DevServerAgentClient) *EstablishConnection {
	return &EstablishConnection{sshTargets: sshTargets, devServers: devServers, conns: conns, agent: agent}
}

type EstablishConnectionInput struct {
	SshTargetID string
}

func (uc *EstablishConnection) Execute(ctx context.Context, in EstablishConnectionInput) (domain.Connection, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	target, err := uc.sshTargets.Get(ctx, tenantID, in.SshTargetID)
	if err != nil {
		return domain.Connection{}, err
	}

	// Find-or-create the DevServer row this SSH target backs — an SSH
	// target only becomes routable once it's the ssh_target_id of a
	// relay-ssh-mode DevServer. ID generation happens here, not in
	// postgres/, matching register_dev_server.go's own convention.
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, target.ID)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_RESOLVE_FAILED", "failed to resolve dev server for ssh target", err)
	}
	if !found {
		devServer, err = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID)
		if err != nil {
			return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_CONSTRUCT_FAILED", "failed to construct dev server for ssh target", err)
		}
		devServer, err = uc.devServers.Register(ctx, devServer)
		if err != nil {
			return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_REGISTER_FAILED", "failed to register dev server for ssh target", err)
		}
	}

	// The handshake itself — bootstrap/deploy is a separate concern if the
	// relay binary isn't deployed yet; Health() here confirms an
	// already-bootstrapped target is actually reachable before the
	// Connection is marked established. Per infra-fleet-service.md §8's
	// deadline rule, the caller (gRPC handler, TASK-164 below) carries an
	// explicit timeout longer than the intra-cluster default.
	reachable, err := uc.agent.Health(ctx, devServer)
	if err != nil || !reachable {
		return domain.Connection{}, apperrors.New(apperrors.KindUnavailable, "INFRA_SSH_CONNECT_FAILED", "failed to establish SSH connection to target", err)
	}

	conn, err := domain.NewConnection(uuid.NewString(), tenantID, devServer.ID, "", "")
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_CONNECTION_CONSTRUCT_FAILED", "failed to construct connection", err)
	}
	conn.Status = "established"
	return uc.conns.CreateConnection(ctx, conn)
}
```

## `internal/adapter/grpc/server.go`

Add `listSshTargets *usecase.ListSshTargets`, `getSshState
*usecase.GetSshState`, `establishConnection *usecase.EstablishConnection`
fields to `Server` and `New(...)`'s parameter list, following the existing
pattern.

```go
func (s *Server) ListSshTargets(ctx context.Context, req *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
	targets, err := s.listSshTargets.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.SshTarget, 0, len(targets))
	for _, t := range targets {
		out = append(out, &infrafleetv1.SshTarget{
			Id: t.ID, TenantId: t.TenantID, Host: t.Host, User: t.UserName, VaultSshRole: t.VaultSSHRole,
		})
	}
	return &infrafleetv1.ListSshTargetsResponse{SshTargets: out}, nil
}

func (s *Server) GetSshState(ctx context.Context, req *infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error) {
	state, err := s.getSshState.Execute(ctx, usecase.SshStateInput{SshTargetID: req.GetSshTargetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetSshStateResponse{Connected: state.Connected, ConnectionId: state.ConnectionID}
	if state.LastActivity != nil {
		resp.LastActivityUnixMs = state.LastActivity.UnixMilli()
	}
	return resp, nil
}

func (s *Server) EstablishConnection(ctx context.Context, req *infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error) {
	conn, err := s.establishConnection.Execute(ctx, usecase.EstablishConnectionInput{SshTargetID: req.GetSshTargetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.Connection{Id: conn.ID, DevServerId: conn.DevServerID, Status: conn.Status}
	if conn.LastActivityAt != nil {
		resp.EstablishedAtUnixMs = conn.LastActivityAt.UnixMilli()
	}
	return resp, nil
}
```

## `cmd/server/main.go`

Construct `usecase.NewListSshTargets(sshTargetStore)`,
`usecase.NewGetSshState(sshTargetStore, repo, repo)`,
`usecase.NewEstablishConnection(sshTargetStore, repo, repo, agentClient)`
next to this service's existing usecase constructors, and pass them into
`grpc.New(...)`'s call in `Server`'s field order.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./... && go vet ./...
```
