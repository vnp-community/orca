# TASK-INT-03-02: Expose captured `NodeVersion` via `ResolveConnectionResponse`

**From Solution:** SOL-INT-03
**Priority:** P2 — genuine gap, but not required for the primary merge-algorithm fix (TASK-INT-03-01/03) to land
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Neither Part A's nor Part B's `preflight.check` reports a Node version, and
the two Parts don't even agree on `preflight.check`'s own shape (confirmed
Part A/Part B divergence, `infra-fleet-service.md` §10) — so SOL-INT-03
deliberately does NOT call `preflight.check` generically for this. Node
version is already captured at connect time
(`devserveragent.HandshakeInfo.NodeVersion`, populated from
`inboundHandshakeParams.NodeVersion` at `agentwsserver/server.go:53,162`)
but no RPC exposes it. This adds a small, read-only, additive field to an
RPC already called on every preflight run.

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend
`ResolveConnectionResponse` (`:158-175`):

```protobuf
message ResolveConnectionResponse {
  bool connected = 1;
  DevServer dev_server = 2;
  string repo_path = 3;
  string worktree_id = 4;
  string connection_id = 5;
  // node_version is the connected session's self-reported Node.js version
  // (devserveragent.HandshakeInfo.NodeVersion, captured at handshake time)
  // — empty when connected is false or the session predates this field.
  // Added for SOL-INT-03's preflight merge; not populated by any other RPC.
  string node_version = 6;
}
```

In `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`'s
`ResolveConnection` handler, populate it from the resolved session's
`HandshakeInfo` — the live session lookup needs a small port addition
(`devserveragent.Client` already holds `HandshakeInfo` per session, see
`session.go:62`; expose it via a narrow read method, e.g.
`Client.HandshakeInfoFor(devServerID string) (devserveragent.HandshakeInfo, bool)`,
mirroring `LiveSessionCount`'s (TASK-AWS-02-03) mutex-guarded read shape).
Wire that into `usecase.ResolveConnection` as an optional port so a
connection with no live session (or an older field-less session) leaves
`NodeVersion` empty rather than erroring.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./...
```

Expected: clean build; `buf breaking` reports only an additive field.
