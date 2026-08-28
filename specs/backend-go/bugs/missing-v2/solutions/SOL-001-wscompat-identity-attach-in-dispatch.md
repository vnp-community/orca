# SOL-001: Fix BUG-001 — attach caller identity centrally in `Registry.Dispatch`, not per-handler

**Resolves:** BUG-001
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/registry.go`, `internal/adapter/wscompat/channels_emulator_folderworkspace_host.go` (net simplification — 5 call sites lose their now-redundant per-handler code, if any is added ad hoc before this lands)
**Priority:** High
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

Two target-architecture statements this fix follows directly:

- `services/api-gateway.md` §2 (Bounded context): *"It needs to correctly
  propagate tenant/user identity on every call, via a validated JWT/session
  translated into gRPC metadata... a sufficient substitute for what process
  forking bought the TS system."* — identity propagation is framed as **one
  gateway-wide job**, not something each route/handler independently
  implements.
- `architecture/08-inter-service-communication.md` "gRPC conventions":
  *"Server-side interceptors (shared via `orca-go-common`) handle: JWT
  validation, tenant-context extraction... No service hand-rolls this
  per-RPC."* — the target design's explicit position on this exact class of
  bug: a cross-cutting concern belongs in one shared place, not duplicated
  per call site where it can (and, per BUG-001, did) get forgotten.

`wscompat`'s current pattern — every channel handler individually calls
`ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{...})` before its
gRPC client call — is the client-side mirror of that same anti-pattern.
BUG-001 is exactly the failure mode both docs are warning against: a
handler (5, actually — every `folderWorkspace.*` registration) simply
forgot the line. The fix moves identity attachment to the one place every
dispatched call already passes through: `Registry.Dispatch`.

## Design

`Registry.Dispatch(ctx, id Identity, channel string, args []json.RawMessage)`
(the shared entry point every `ChannelHandler` is invoked through — visible
in `channels_repo_ssh_status_workspace_test.go`'s `r.Dispatch(context.Background(), Identity{...}, "repo.list", ...)`
calls) attaches identity to `ctx` once, before calling the registered
handler, instead of leaving it to each handler:

```go
// registry.go — Dispatch, sketch
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	handler, ok := r.handlers[channel]
	if !ok {
		return nil, fmt.Errorf("channel %q is not yet implemented...", channel)
	}
	// NEW: attach once, here — every handler's outbound gRPC call inherits
	// it automatically via ctx propagation, matching 08's "no service
	// hand-rolls this per-RPC" principle applied to this client's own
	// dispatch boundary.
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	return handler(ctx, id, args)
}
```

Every existing handler's own `ctx = gatewaygrpc.AttachIdentity(...)` line
becomes redundant (harmless — `AttachIdentity` overwrites, doesn't merge —
but should be deleted in the same change for clarity, not left as dead
duplication). The 5 `folderWorkspace.*` handlers need no per-handler change
at all once this lands; they inherit identity automatically, closing
BUG-001 without touching that file.

### Timeout, folded into the same fix

BUG-001's report also noted the 5 `folderWorkspace.*` handlers skip the
`context.WithTimeout(ctx, rpcTimeout)` every sibling handler applies —
`08-inter-service-communication.md`'s "Deadlines are mandatory on every
outbound call... no unbounded gRPC call exists anywhere in the system" is
unambiguous that this is also a bug, not a style choice. Fold the same
per-call timeout into `Dispatch` alongside identity attachment, for the
same "one shared place, not N call sites" reason:

```go
ctx, cancel := context.WithTimeout(ctx, rpcTimeout) // or a per-channel override, see below
defer cancel()
```

One nuance: some existing handlers use a channel-group-specific timeout
constant (`repoSSHStatusWorkspaceRPCTimeout` vs. the shared `rpcTimeout`) —
`Dispatch` needs either a single system-wide default (simplest, and per
08's own default-5s-intra-cluster-call framing, defensible) or a
per-registration override (`Register(name, handler, opts...)` gaining a
`WithTimeout(d)` option). Recommend the single default first — the
per-group constants that exist today don't appear to differ for a
documented reason; verify that before adding override complexity that
matches the general codebase preference (per `03-clean-architecture-guidelines.md`'s
"YAGNI over speculative flexibility" framing, cited elsewhere in this
service's own files) for the simplest option that satisfies the actual
requirement.

## Testing Plan

- **Regression test for the actual bug**: a fake `project-service` client
  whose `ListFolderWorkspaces` (and the other 4 methods) asserts identity
  is present in the incoming `ctx` (e.g. read back via whatever
  `gatewaygrpc.AttachIdentity`'s counterpart extraction helper is,
  mirroring how the real server-side interceptor would) — fails loudly if
  absent. This is the test BUG-001's own report flagged as missing:
  asserting on `ctx`, not just the request struct shape.
- **Structural regression guard**: a table-driven test enumerating every
  registered channel name and asserting `Dispatch` attaches identity for
  each — since this fix makes it a `Dispatch`-level guarantee, one test
  covers all current AND future channels, closing the "N handlers, N
  chances to forget" failure class permanently rather than per-namespace.
- Re-run `tests/client/rpc-catalog.spec.ts`'s `folderWorkspace.list` case
  live against the fixed deployment — should move from `PROJECT_NO_TENANT`
  to either success or a legitimate empty-tenant-data response.
