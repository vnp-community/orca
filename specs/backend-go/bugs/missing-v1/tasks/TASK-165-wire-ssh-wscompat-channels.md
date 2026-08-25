# TASK-165: Register `ssh.listTargets`/`ssh.getUserAccount`/`ssh.getState`/`ssh.connect` `wscompat` channels

**From Solution:** SOL-024
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-162 (proto stubs), TASK-164 (backing RPCs)
**Status:** `[partial]` `registerSshChannels` (ssh.listTargets/getUserAccount/getState/connect, with the 20s `ssh.connect` deadline override) implemented in the new `channels_repo_ssh_status_workspace.go` file, not `channels.go`. Builds/tests green in isolation; not wired into production `RegisterRealChannels` — see TASK-151's status note for the integration line (no new client dial needed, `infraFleetClient` already exists in `main.go`).

---

## Context

`ssh.getUserAccount` in the old backend was never a distinct "Linux
account provisioning" concept — it just read the target's configured
username, so it derives from `ListSshTargets` client-side rather than
getting its own RPC.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Add, mirroring `registerDevServerChannels`'s exact shape:

```go
// ── ssh.* ────────────────────────────────────────────────────────────────
func registerSshChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("ssh.listTargets", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetSshTargets(), nil
	})

	r.Register("ssh.getUserAccount", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getUserAccountArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[getUserAccountArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
		if err != nil {
			return nil, err
		}
		for _, t := range resp.GetSshTargets() {
			if t.GetId() == in.SshTargetID {
				return map[string]string{"username": t.GetUser()}, nil
			}
		}
		return nil, fmt.Errorf("ssh target %q not found", in.SshTargetID)
	})

	r.Register("ssh.getState", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getStateArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[getStateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetSshState(rpcCtx, &infrafleetv1.GetSshStateRequest{SshTargetId: in.SshTargetID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("ssh.connect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type connectArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[connectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Longer-than-rpcTimeout deadline: this is an SSH handshake, not a
		// Postgres read — infra-fleet-service.md §8's "explicit timeout
		// distinct from the default 5s intra-cluster gRPC deadline" rule,
		// same reasoning as BootstrapFleetTarget's streaming RPC.
		rpcCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		resp, err := client.EstablishConnection(rpcCtx, &infrafleetv1.EstablishConnectionRequest{SshTargetId: in.SshTargetID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

`fmt` needs to be added to this file's import block if not already
present.

Register from `RegisterRealChannels`, next to `registerDevServerChannels`:

```go
	registerDevServerChannels(r, infraFleetClient)
	registerSshChannels(r, infraFleetClient) // NEW
```

No new gRPC client dial needed — `infraFleetClient` is already threaded
through.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
