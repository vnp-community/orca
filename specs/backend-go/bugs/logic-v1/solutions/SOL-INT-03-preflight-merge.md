# SOL-INT-03: `PreflightCheckResult` + local/relay merge, reusing already-wired `GetFleetHealth`/`ScanWorkspacePorts`/`Relay`

**Resolves:** [BUG-INT-03](../BUG-INT-03-preflight-merge-not-implemented.md)
**Service:** `api-gateway` (usecase + wscompat) — no new backend service
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/usecase/preflight.go` (new: `PreflightCheckResult`, `MergePreflightStatuses`)
- `backend-go/services/api-gateway/internal/usecase/preflight_test.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`registerPreflightChannels` rewritten)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go` (extended)
- **Agent (`agent/`) changes**: none required beyond [SOL-INT-01](./SOL-INT-01-cli-auth-proxy.md)'s already-flagged relay-ssh gap, which this solution inherits rather than duplicates
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD, and grounded in what already exists)

BUG-INT-03's finding — zero `PreflightCheckResult`/merge-algorithm/
`relay-connectivity` concept anywhere in backend-go — is accurate. But
three of BL-INT-03's four remote-check categories are not new relay work
this solution has to build: they map directly onto RPCs `infra-fleet-service`
already exposes and `wscompat` already calls successfully elsewhere:

| BL-INT-03 remote check | Already-wired RPC | Existing caller |
|---|---|---|
| `gh`/`glab` auth status | `InfraFleetService.Relay` → `github.auth.status`/`gitlab.auth.status` | [SOL-INT-01](./SOL-INT-01-cli-auth-proxy.md) (new, this bug's sibling) |
| Disk space | `InfraFleetService.GetFleetHealth` → `DiskUsagePercent` | `channels.go`'s existing `fleet.health.checkAll` handler |
| Port availability | `InfraFleetService.ScanWorkspacePorts` | `channels_repo_ssh_status_workspace.go:481-502`'s existing `workspacePorts.scan` |
| Node version | Not yet exposed as a queryable fact — see "Genuine gap" below | — |

This means the bulk of BUG-INT-03's remaining work is the **merge
algorithm and result type** BL-INT-03 actually centers on
(`mergePreflightStatuses()`), not a from-scratch relay implementation —
the relay plumbing for 3 of 4 categories is reuse, and the fourth
(auth status) is this bug's sibling, [SOL-INT-01](./SOL-INT-01-cli-auth-proxy.md).

### Genuine gap: Node version

Neither `preflight.check` (Part A: `fs-agent-extensions.ts::handlePreflightCheck`,
a `{services:[]string]} -> {[service]:bool}` binary-availability check for
`github-cli`/`ripgrep`/`docker`/`claude`; Part B:
`preflight-handler.ts::checkFullPreflight`, a no-param
`{platform,gh,glab,git}` check) reports a Node version, **and the two
Parts don't even agree on `preflight.check`'s own shape** — exactly the
Part A/Part B divergence `infra-fleet-service.md` §10 warns about,
confirmed here by reading both handlers directly rather than assuming a
flat namespace. Calling `preflight.check` generically through `Relay`
would silently get a different response shape depending on which
transport mode the target Dev Server uses — this solution deliberately
does **not** call `preflight.check` at all, for exactly this reason
(narrower, unambiguous RPCs are used instead, per the table above).

For Node version specifically: `devserveragent.HandshakeInfo.NodeVersion`
(`agentwsserver/server.go:53`, populated from the agent's own
`agent.handshake` params — `inboundHandshakeParams.NodeVersion`) is
**already captured** at connect time and held on the live session, but
`InfraFleetService`'s API surface has no RPC exposing it. This solution
proposes extending `ResolveConnectionResponse`
(`infrafleet.proto:158-175`) with an optional `node_version` field sourced
from the resolved connection's session metadata — a small, low-risk proto
addition (read-only, additive field on an RPC already called on every
preflight run), flagged explicitly as a scope addition rather than
silently assumed to exist.

## Design — `PreflightCheckResult` type and merge algorithm

```go
// internal/usecase/preflight.go
type PreflightStatus string

const (
    PreflightOK      PreflightStatus = "ok"
    PreflightWarning PreflightStatus = "warning"
    PreflightError   PreflightStatus = "error"
    PreflightSkip    PreflightStatus = "skip"
)

type PreflightSource string

const (
    PreflightSourceLocal PreflightSource = "local"
    PreflightSourceRelay PreflightSource = "relay"
)

type PreflightCheckResult struct {
    ID      string
    Status  PreflightStatus
    Message string
    Details map[string]any
    Source  PreflightSource
}

// MergePreflightStatuses seeds by id from local, then overrides by id from
// relay (relay wins conflicts, matches BL-INT-03's documented precedence),
// appends any relay-only ids, and appends a relay-connectivity warning
// when relayErr is non-nil instead of any relay results at all — the
// direct Go port of mergePreflightStatuses().
func MergePreflightStatuses(local, relay []PreflightCheckResult, relayErr error) []PreflightCheckResult {
    merged := make(map[string]PreflightCheckResult, len(local)+len(relay))
    order := make([]string, 0, len(local)+len(relay)+1)
    for _, c := range local {
        merged[c.ID] = c
        order = append(order, c.ID)
    }
    if relayErr != nil {
        order = append(order, "relay-connectivity")
        merged["relay-connectivity"] = PreflightCheckResult{
            ID: "relay-connectivity", Status: PreflightWarning,
            Message: "Cannot reach Dev Server — showing local checks only",
            Source:  PreflightSourceLocal,
        }
    } else {
        for _, c := range relay {
            if _, existed := merged[c.ID]; !existed {
                order = append(order, c.ID)
            }
            merged[c.ID] = c // relay overrides local by id
        }
    }
    out := make([]PreflightCheckResult, 0, len(order))
    for _, id := range order {
        out = append(out, merged[id])
    }
    return out
}
```

## Design — where the merge logic lives (a flagged tension with `api-gateway.md` §2)

`api-gateway.md` §2 is explicit that gateway-level "cross-service response
orchestration" is out of bounds: "if a REST call needs data from two
services, that composition belongs to the calling client or to a service
exposing a composed read, not to a gateway orchestration layer." Combining
a local check with a relay-sourced check by id is exactly that shape of
composition, and is called out here rather than glossed over.

This solution places `MergePreflightStatuses` in `api-gateway`'s
`usecase/` anyway, as a **bounded, explicitly-flagged exception**, for two
reasons specific to this handler and not a general license to add
orchestration elsewhere:

1. **No service in the 17-service catalog owns "preflight status" as a
   domain concept.** It isn't `infra-fleet-service`'s (which owns
   reachability/health, not "is `gh` authenticated"), and it isn't
   `scm-integration-service`'s (which is deliberately not a CLI wrapper,
   per this bug's own "What backend-go has" section). Composing it
   somewhere requires either a new, single-purpose service for one
   read-only aggregate — a heavier fix than the gap justifies — or the
   caller (frontend) doing the merge itself, which BL-INT-03 explicitly
   does *not* want (the merge is meant to be a single trusted answer, not
   client-reimplemented per platform).
2. **`registerPreflightChannels` already lives directly in `wscompat` as a
   local, no-owning-service computation**, and this file's own package
   already tolerates that shape elsewhere when no service is a good fit —
   see `registerCrashReportChannels`'s doc comment (`channels.go`:
   "there is genuinely nothing to report from backend-go's crash/panic
   path... not a stub") and `registerRateLimitChannels` (exposing
   in-process gateway config). This solution's merge is more elaborate
   than those two, which is exactly why it's flagged here rather than
   assumed to be more of the same — if this namespace grows further
   (more remote checks, more merge complexity), promoting it to a real
   owning service should be revisited, not layered on indefinitely.

## Design — local checks

```go
func localPreflightChecks() []PreflightCheckResult {
    return []PreflightCheckResult{
        // Unchanged from today's hardcoded literal — git-gateway-service's
        // local executor requires the real git binary in its own
        // container image, an infra guarantee, not a per-request probe;
        // now explicitly labeled source:"local" rather than an
        // undifferentiated map key.
        {ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
    }
}
```

Local integration-token-format/network-reachability checks from BL-INT-03
are deliberately **not** added in this pass — `credentials.*`
([SOL-INT-02](./SOL-INT-02-credentials-channels-wiring.md)) gives a
"configured" signal per service but token *format* validation has no
existing hook, and a network-reachability ping has no established
precedent anywhere in backend-go to model after. Flagged as a smaller,
independent follow-up rather than blocking this merge-algorithm fix on it.

## Design — relay checks + wscompat wiring

```go
// registerPreflightChannels — rewritten
func registerPreflightChannels(r *Registry, infraClient infrafleetv1.InfraFleetServiceClient) {
    r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type preflightArgs struct {
            ConnectionID string `json:"connectionId"` // empty = local-only, no relay attempted
        }
        in, _ := decodeArg[preflightArgs](args, 0)
        local := localPreflightChecks()
        if in.ConnectionID == "" {
            return usecase.MergePreflightStatuses(local, nil, nil), nil
        }

        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()

        relay, relayErr := runRelayPreflightChecks(rpcCtx, infraClient, in.ConnectionID, id.UserID)
        return usecase.MergePreflightStatuses(local, relay, relayErr), nil
    })
}

// runRelayPreflightChecks fans out the 3 already-wired relay calls
// concurrently; a hard failure on GetFleetHealth/ScanWorkspacePorts (the
// connectivity-defining calls) is treated as relay-unreachable for the
// whole batch — an auth-status-specific failure (e.g.
// ErrAgentMethodNotFound on a relay-ssh Dev Server, see SOL-INT-01) is
// NOT connectivity-wide and instead produces one skip-status result for
// just that check, so a relay-ssh Dev Server still gets disk/port results
// merged in.
func runRelayPreflightChecks(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, connectionID, userID string) ([]usecase.PreflightCheckResult, error) {
    health, err := client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{})
    if err != nil {
        return nil, err // relay-connectivity warning, no partial results
    }
    var results []usecase.PreflightCheckResult
    for _, h := range health.GetStatuses() {
        results = append(results, usecase.PreflightCheckResult{
            ID: "disk-space", Source: usecase.PreflightSourceRelay,
            Status:  diskStatus(h.GetDiskPercent()),
            Message: fmt.Sprintf("%.0f%% disk used", h.GetDiskPercent()),
        })
    }

    if ports, err := client.ScanWorkspacePorts(ctx, &infrafleetv1.ScanWorkspacePortsRequest{ConnectionId: connectionID}); err == nil {
        results = append(results, usecase.PreflightCheckResult{
            ID: "port-availability", Source: usecase.PreflightSourceRelay, Status: usecase.PreflightOK,
        })
    }

    for _, c := range []struct{ id, method string }{{"github-cli-auth", "github.auth.status"}, {"gitlab-cli-auth", "gitlab.auth.status"}} {
        paramsJSON, _ := json.Marshal(map[string]any{"userId": userID})
        resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{ConnectionId: connectionID, Method: c.method, ParamsJson: string(paramsJSON)})
        if err != nil {
            // Per-check skip, not connectivity-wide — see this function's
            // doc comment. Covers SOL-INT-01's relay-ssh gap honestly.
            results = append(results, usecase.PreflightCheckResult{ID: c.id, Status: usecase.PreflightSkip, Message: "not available on this connection mode", Source: usecase.PreflightSourceRelay})
            continue
        }
        var out struct{ OK bool `json:"ok"` }
        _ = json.Unmarshal([]byte(resp.GetResultJson()), &out)
        status := usecase.PreflightError
        if out.OK {
            status = usecase.PreflightOK
        }
        results = append(results, usecase.PreflightCheckResult{ID: c.id, Status: status, Source: usecase.PreflightSourceRelay})
    }
    return results, nil
}
```

## Test plan

- `usecase/preflight_test.go` — `MergePreflightStatuses`: relay overrides
  local by id; relay-only ids are appended; a non-nil `relayErr` produces
  exactly the `relay-connectivity` warning and **no** relay results, even
  if a non-empty `relay` slice was passed (defends the "local checks only"
  contract); output order is stable (local order, then relay-only
  appends, then the warning) for deterministic UI rendering.
- `wscompat/channels_test.go` — `preflight.check` with no `connectionId`
  returns local-only results, no RPC calls made.
- `wscompat/channels_test.go` — `GetFleetHealth` failing produces the
  `relay-connectivity` warning and the local `git` check still present.
- `wscompat/channels_test.go` — a `github.auth.status` relay failure
  (simulating a relay-ssh Dev Server) produces a `skip`-status
  `github-cli-auth` result while `disk-space`/`port-availability` still
  come back `ok` — proves per-check-not-connectivity-wide degradation.
- Contract test: every `PreflightCheckResult` in a merged response has a
  non-empty `Source` — regression guard against a future relay check
  forgetting to tag itself.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:551-573` — the stub this solution replaces
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`fleet.health.checkAll` handler, read in full during this audit) — `GetFleetHealth` call/response shape reused verbatim
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:460-502` — existing `workspacePorts.scan`/`ScanWorkspacePorts` wiring reused
- `agent/src/relay/fs-agent-extensions.ts:262-303` — Part A's `preflight.check` (`handlePreflightCheck`), confirming the Part A/Part B shape divergence this solution avoids depending on
- `agent/src/relay/preflight-handler.ts:206-224` — Part B's `preflight.check` (`checkFullPreflight`), the second, different shape
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:53` — `inboundHandshakeParams.NodeVersion`, already-captured data this solution's proposed `node_version` field would expose
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:139-175` — `ResolveConnectionResponse`, proposed extension point for Node version
- `specs/backend-go/tdd/services/api-gateway.md:31-39` (§2, cross-service composition boundary — the tension this solution flags and bounds), `:576-591` (crashReports/rateLimits precedent for local-answer channels)
- `specs/backend-go/tdd/services/infra-fleet-service.md:560-573` (§10, Part A/Part B divergence warning)
- `docs/logic/remote-integration/BL-INT-03-preflight-merge.md` — merge algorithm, `PreflightCheckResult` schema, `relay-connectivity` warning
- [SOL-INT-01](./SOL-INT-01-cli-auth-proxy.md) — `github.auth.status`/`gitlab.auth.status` relay this solution consumes
- [SOL-INT-02](./SOL-INT-02-credentials-channels-wiring.md) — noted as the not-yet-connected local-token-format-check follow-up
