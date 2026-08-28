# SOL-INT-01: Wire `github.checkAuthStatus`/`gitlab.checkAuthStatus`/`github.startAuthLogin` onto infra-fleet-service's existing `Relay` RPC

**Resolves:** [BUG-INT-01](../BUG-INT-01-cli-auth-proxy-not-implemented.md)
**Service:** `api-gateway` (wscompat wiring only) — no `infra-fleet-service` proto/usecase changes needed
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli_auth.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli_auth_test.go` (new)
- **Agent (`agent/`) changes**: none required for direct-websocket/relay-websocket Dev Servers; a small, low-risk addition needed for relay-ssh Dev Servers — see "Agent changes" below
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD, and grounded in `agent/`'s real state)

BUG-INT-01's own finding is correct as far as it goes — `preflight.check`
(`channels.go:565-573`) is a hardcoded local literal with no downstream
call, and there is no login-flow endpoint in `wscompat`. But its framing
("the backend never contacts any Dev Server... no relay of any kind
exists") undercounts what's already built one layer down, in
`infra-fleet-service`, which this fix reuses directly rather than building
new infrastructure:

- **The relay mechanism already exists and is already reused four times.**
  `InfraFleetService.Relay` (`infrafleet.proto:196-209`) is a generic
  `(connection_id, method, params_json) -> result_json` passthrough,
  implemented (`adapter/grpc/server.go:188`) over
  `usecase.Relay.Execute` → `devserveragent.Client.Exec` — the exact
  generic dispatch `infra-fleet-service.md` §1 describes as this
  service's "provider registry: a `connectionId`-keyed dispatch table."
  `wscompat` already calls it from `channels_accounts.go`,
  `channels_nativechat.go`, `channels_browser.go`, and
  `channels_browser_profiles.go` — this solution is a fifth, same-shaped
  caller, not a new pattern.
- **The agent-side functionality this bug asks for already exists and is
  already real**, contradicting BUG-INT-01's framing that "the CLI tools
  live on the Dev Server... [and nothing] proxies preflight/auth checks
  there":
  - `agent/src/relay/agent-rpc-dispatch.ts:1029-1047` implements
    `github.auth.status`/`gitlab.auth.status` for real, delegating to
    `agent/src/relay/external-api-connector.ts:359-390`'s
    `handleGitHubAuthStatus`/`handleGitLabAuthStatus`, which run
    `gh auth status`/`glab auth status` and return `{ok, stdout, stderr}`.
  - Per-user config-dir isolation — BL-INT-01's exact requirement
    (`GH_CONFIG_DIR=~/.config/gh/<userId>/`,
    `GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/`) — is already
    implemented, not missing: `external-api-connector.ts:118-131`'s
    `buildGhEnv(userId, ...)`/`buildGlabEnv(userId, ...)` build exactly
    those two env vars, and both auth-status handlers call them
    (`external-api-connector.ts:366`: `const env = buildGhEnv(userId,
    config.toolEnv)`).
  - The interactive login flow (BL-INT-01's "run `gh auth login` over a
    relayed PTY") is *also* already implemented, via the existing generic
    PTY-spawn path, not a dedicated login RPC:
    `agent/src/relay/pty-handler.ts:680-699` special-cases a PTY `command`
    starting with `"gh "`/`"glab "` plus a `userId` param, and applies
    `buildGhEnv`/`buildGlabEnv` to that PTY's environment (comment at
    `:687-690`: "gh/glab auth-login PTYs need the same per-user
    `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` isolation as the `github.*`/`gitlab.*`
    RPC handlers... otherwise every user's `gh auth login` shares one
    default config on the Dev Server").

BUG-INT-01 was written correctly against `wscompat` (nothing was wired
there) but didn't have visibility into `agent/`'s already-shipped side —
this is squarely a wiring gap, not a "build the feature" gap. That
reframing changes the shape of the fix from "implement an SSH-relay proxy
and CLI invocation from scratch" to "add three thin `wscompat` channels
onto existing, tested RPCs."

## What is a genuine gap: Part A / Part B divergence

`infra-fleet-service.md` §10 flags this exact failure mode by name
("Known TS drift to account for during porting... the agent process runs
**two independently-implemented** RPC surfaces... that frequently diverge
in method names and param shapes... `adapter/devserveragent/` must model
this as two distinct method-call surfaces... not assume a single flat
namespace"). It applies here directly: `github.auth.status`,
`gitlab.auth.status`, and the PTY `userId`-scoped env injection are all
implemented **only** in Part A (`agent-rpc-dispatch.ts`, used by
direct-websocket and relay-websocket connection modes). A repo-wide check
of Part B (`agent/src/relay/relay.ts`'s `RelayDispatcher`,
`preflight-handler.ts`) turns up zero occurrences of `github.auth.status`
or `gitlab.auth.status` — relay-ssh Dev Servers cannot answer either
call today.

This solution does not paper over that gap; it surfaces it as an explicit,
typed failure so the merge logic in
[SOL-INT-03](./SOL-INT-03-preflight-merge.md) can degrade honestly (report
"not available on this connection mode" rather than a false negative), the
same posture SOL-009 took for `RenameFile`/`CopyFile`'s relay gap.

## Design — wscompat channels

```go
// channels_cli_auth.go
//
// github.checkAuthStatus / gitlab.checkAuthStatus / github.startAuthLogin
// relay through infra-fleet-service's existing generic Relay RPC — see
// SOL-INT-01. AGENT-SIDE WORK ALREADY LANDED (unlike SOL-004's
// accounts.*): agent/src/relay/agent-rpc-dispatch.ts already implements
// github.auth.status/gitlab.auth.status with real per-user GH_CONFIG_DIR/
// GLAB_CONFIG_DIR isolation; this file only adds the missing wscompat
// registration. Requires connectionId, same open prerequisite as
// accounts.* (TASK-023) — see cliAuthArgs's doc comment.
package wscompat

type cliAuthArgs struct {
    ConnectionID string `json:"connectionId"`
}

func registerCLIAuthChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    registerCLIAuthStatusRelay(r, client, "github.checkAuthStatus", "github.auth.status")
    registerCLIAuthStatusRelay(r, client, "gitlab.checkAuthStatus", "gitlab.auth.status")
    registerCLIAuthLoginChannel(r, client, "github.startAuthLogin", "gh auth login --hostname github.com --web")
    registerCLIAuthLoginChannel(r, client, "gitlab.startAuthLogin", "glab auth login --hostname gitlab.com --web")
}

// registerCLIAuthStatusRelay is the shared shape for both status checks —
// same pattern as channels_accounts.go's registerAccountsRelay.
func registerCLIAuthStatusRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
    r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        in, err := decodeArg[cliAuthArgs](args, 0)
        if err != nil {
            return nil, err
        }
        if in.ConnectionID == "" {
            return nil, fmt.Errorf("CLI_AUTH_NO_CONNECTION: connectionId is required")
        }
        paramsJSON, _ := json.Marshal(map[string]any{"userId": id.UserID})
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
            ConnectionId: in.ConnectionID, Method: agentMethod, ParamsJson: string(paramsJSON),
        })
        if err != nil {
            // ErrAgentMethodNotFound (relay-ssh Dev Servers, see this
            // file's package doc comment) surfaces here — mapped to a
            // typed, non-crashing "unavailable" result by SOL-INT-03's
            // merge layer, not swallowed here.
            return nil, err
        }
        var out map[string]any
        if err := json.Unmarshal([]byte(resp.GetResultJson()), &out); err != nil {
            return nil, err
        }
        return out, nil // {ok, stdout, stderr} verbatim from external-api-connector.ts
    })
}

// registerCLIAuthLoginChannel spawns an interactive `gh`/`glab auth login`
// PTY via the ALREADY-existing SpawnTerminalSession RPC — not a new agent
// RPC. UserId is forwarded so pty-handler.ts's existing command-prefix
// match (`gh `/`glab `) engages the same per-user env isolation the status
// checks use. The frontend attaches to the returned ptyId via the normal
// terminal.* WS stream (infrafleet.proto's AttachPty), unchanged by this
// solution.
func registerCLIAuthLoginChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, command string) {
    r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        in, err := decodeArg[cliAuthArgs](args, 0)
        if err != nil {
            return nil, err
        }
        if in.ConnectionID == "" {
            return nil, fmt.Errorf("CLI_AUTH_NO_CONNECTION: connectionId is required")
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.SpawnTerminalSession(rpcCtx, &infrafleetv1.SpawnTerminalSessionRequest{
            ConnectionId: in.ConnectionID,
            Command:      command,
            UserId:       id.UserID, // engages pty-handler.ts's buildGhEnv/buildGlabEnv branch
        })
        if err != nil {
            return nil, err
        }
        return resp, nil // { ptyId, ... } — frontend attaches over the existing terminal WS stream
    })
}
```

`RegisterRealChannels` gains `registerCLIAuthChannels(r, infraClient)` —
`infraClient` is already constructed for `registerAccountsChannels` et al.,
no new dial.

## Agent (`agent/`) changes needed

**None** for Dev Servers on `direct-websocket`/`relay-websocket` mode —
`github.auth.status`, `gitlab.auth.status`, and the PTY `userId` isolation
are already implemented and shipping.

**A small, explicitly-flagged change is needed for relay-ssh Dev Servers**
to close the Part A/Part B gap above: add `github.auth.status`/
`gitlab.auth.status` cases to Part B's dispatcher
(`agent/src/relay/relay.ts` / wherever `RelayDispatcher` registers its
method table), reusing `external-api-connector.ts`'s existing
`handleGitHubAuthStatus`/`handleGitLabAuthStatus`/`buildGhEnv`/
`buildGlabEnv` verbatim (that module is already transport-agnostic — it's
imported by Part A's dispatcher today, and nothing in its signature ties
it to Part A specifically). This is copy-adjacent reuse of already-tested
code, not new logic, but it is still an `agent/` change and is called out
per this task's instruction rather than silently assumed. Until that
lands, relay-ssh Dev Servers get a typed "not available on this connection
mode" result from [SOL-INT-03](./SOL-INT-03-preflight-merge.md)'s merge
layer (via `domain.ErrAgentMethodNotFound`, already surfaced by
`devserveragent.Client.Exec`, `client.go:238-260`) — an honest degradation,
not a silent wrong answer, matching SOL-009's precedent for
`RenameFile`/`CopyFile`'s relay gap.

## Test plan

- `wscompat/channels_cli_auth_test.go` — `github.checkAuthStatus`/
  `gitlab.checkAuthStatus` relay to the correct agent method name with
  `{"userId": ...}` params, using a fake `InfraFleetServiceClient`
  (mirrors `channels_accounts_test.go`'s `fakeAccountsRelayClient`
  pattern); missing `connectionId` fails fast with no RPC call.
- `wscompat/channels_cli_auth_test.go` — `github.startAuthLogin` calls
  `SpawnTerminalSession` with the expected `Command` and `UserId` fields
  set from `Identity.UserID`, not from client-supplied args (regression
  guard against a caller spoofing another user's isolated config dir).
- `wscompat/channels_cli_auth_test.go` — a `Relay` call returning
  `codes.NotFound`/`domain.ErrAgentMethodNotFound`-shaped error (simulating
  a relay-ssh Dev Server) propagates as a typed error, not a panic or a
  false `{ok:false}`.

## References

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:196-209` — `RelayRequest`/`RelayResponse`, already-generic passthrough
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:238-260` — `Client.Exec`, `domain.ErrAgentMethodNotFound` typed distinction
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go:1-13,113-142` — `registerAccountsRelay` pattern this solution's channels mirror (SOL-004 precedent)
- `agent/src/relay/agent-rpc-dispatch.ts:1029-1047` — `github.auth.status`/`gitlab.auth.status` case handlers (Part A)
- `agent/src/relay/external-api-connector.ts:1-13,102-131,357-390` — `buildGhEnv`/`buildGlabEnv`, `handleGitHubAuthStatus`/`handleGitLabAuthStatus`
- `agent/src/relay/pty-handler.ts:680-699` — PTY-spawn `gh `/`glab ` command-prefix per-user env injection (the login-flow mechanism this solution reuses)
- `specs/backend-go/tdd/services/infra-fleet-service.md:560-573` (§10, "Known TS drift... two independently-implemented RPC surfaces")
- `specs/backend-go/bugs/missing-v1/tasks/TASK-023-document-accounts-agent-gap.md` — the `connectionId`-prerequisite precedent this solution's channels share
- [SOL-INT-03](./SOL-INT-03-preflight-merge.md) — consumes these channels' underlying `Relay` calls inside the merged `preflight.check` response
