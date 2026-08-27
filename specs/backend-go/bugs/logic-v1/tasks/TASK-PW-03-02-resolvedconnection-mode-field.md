# TASK-PW-03-02: Thread connection `Mode` through `ResolvedConnection`

**From Solution:** SOL-PW-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`, `backend-go/services/git-gateway-service/internal/adapter/grpcclient/resolver.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The five new unary ops and the two streaming ops (TASK-PW-03-01) are
supported by the Dev Server Agent only over `relay-websocket`/
`direct-websocket` connections — `relay-ssh`'s `git.exec` whitelist
explicitly rejects `merge`/`stash`/branch-write subcommands (per
`specs/agent/api/agent-rpc-catalog-git-fs.md`'s Part A/Part B split).
`ResolvedConnection` (`ports.go:33-37`) currently has no field to check
this against, even though `infra-fleet-service.ResolveConnectionResponse`
already returns `dev_server.mode` (`infrafleet.proto:129,171-173`) — this
task threads that existing upstream value through.

## Changes to make

In `internal/usecase/ports.go`:

```go
type ResolvedConnection struct {
	Connected    bool
	ConnectionID string
	RepoPath     string
	// Mode is empty when Connected is false (host-local — the distinction
	// doesn't apply). Populated from infra-fleet-service's
	// ResolveConnectionResponse.dev_server.mode. Added SOL-PW-03 to gate
	// the merge/stash/branch-write/push-stream operations Part B
	// (relay-ssh) genuinely does not support.
	Mode infrafleetv1.ConnectionMode
}
```

(Add the `infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"`
import to `ports.go` if not already present — verify first, since another
file in this package may already need it.)

In `internal/adapter/grpcclient/resolver.go`'s `ResolveConnection`:

```go
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
```

Use the generated `infrafleetv1.ConnectionMode` enum type directly
(`CONNECTION_MODE_RELAY_SSH`/`CONNECTION_MODE_RELAY_WEBSOCKET`/
`CONNECTION_MODE_DIRECT_WEBSOCKET`) rather than a bare string — SOL-PW-03's
prose sketch uses string literals (`"relay-ssh"`) informally; the real
wire type is the enum, and TASK-PW-03-03's sentinel-error check must
compare against `infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH`,
not a string.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/grpcclient/... -run TestResolveConnection -v
```

Expected: clean build; existing `ResolveConnection` tests still pass;
extend/add a case asserting `Mode` is forwarded from
`resp.GetDevServer().GetMode()` when `Connected` is true, and left at its
zero value when `Connected` is false.
