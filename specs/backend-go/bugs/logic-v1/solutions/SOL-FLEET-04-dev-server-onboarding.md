# SOL-FLEET-04: Surface platform detection, add AI-agent detection + remote preflight, close the register-field gap

**Resolves:** [BUG-FLEET-04](../BUG-FLEET-04-dev-server-onboarding-partial.md)
**Service:** `infra-fleet-service` (usecase/domain/proto extensions) +
`api-gateway` (new REST/WS wiring). Steps 1/5/6 already work (see below);
this solution closes Steps 2/3/4/6-field-gap. **Step 5's daemon/HTTP-health
model is [SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md)'s scope, not
re-solved here** — Step 7 depends on
[SOL-FLEET-01](./SOL-FLEET-01-fleet-inventory-import.md).
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go` (Platform/Arch/NodeVersion/AgentVersion — shared with SOL-FLEET-02)
- `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go` (persist handshake info)
- `backend-go/services/infra-fleet-service/internal/usecase/detect_dev_server_agents.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go` (new)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`DevServer` platform fields, `DetectDevServerAgents`, `CheckDevServerPreflight` RPCs, `RegisterDevServerRequest.relay_port`)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (wire the two new RPCs)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `devServer.detectAgents`/`devServer.preflightCheck` channels, extend `toDevServerView`)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

Per `infra-fleet-service.md` §2's bounded-context table, coordination-layer
facts about a dev server (what it is, whether it's reachable, what's on it)
belong to this service — "Knowing which `connectionId` maps to which
host/transport" (`infra-fleet-service.md:65`) is the same category of fact
as "which platform/arch is this host" and "which AI-CLI binaries are on its
PATH": neither is *execution* (which stays on `agent/`, §2's hard boundary,
`infra-fleet-service.md:55-58`), both are *routing/coordination metadata*
this service already owns the pattern for (`DevServer`, `ProviderRegistryEntry`
in §4, `infra-fleet-service.md:142-166`).

**Steps already real, not touched by this solution**: Step 1 (Connect) —
`EstablishConnection` (`backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go:31-77`)
is a genuine SSH+handshake round trip. Step 6's persistence mechanics
(`RegisterDevServer`, `register_dev_server.go:34-50`) are real; only its
*field coverage* has a gap (see below). Step 5's core deploy pipeline is
real (`sshrelay`); its daemon/HTTP-health divergence is
[SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md)'s documented scope, not
re-litigated here.

**A discovered, no-new-`agent/`-work path for Steps 3 and 4**: this
solution's design was almost blocked by BL-FLEET-04's literal wording
("SSH exec `which claude codex gemini openai`", "SSH exec `git --version`"),
which reads as if it needs raw SSH command execution outside the agent
protocol — a capability this service deliberately does not have for
`relay-websocket`/`direct-websocket` dev servers (§2: "PTY, git, and
filesystem **execution** stays on the Dev Server Agent"). Direct
inspection of `agent/src/relay/agent-rpc-dispatch.ts` and
`preflight-handler.ts` confirms two already-real JSON-RPC methods that
close both steps **without any `agent/` change**:

- **Step 3** → `preflight.detectAgents`: params
  `{commands: {id, cmd, requiredCommands?, unsupportedRuntimes?}[]}`, response
  `{agents: string[], platform: string}` — a caller-supplied PATH-lookup
  primitive that "never executes the detected command"
  (`specs/agent/api/agent-rpc-catalog-runtime.md:191-192`), exactly
  BL-FLEET-04 Step 3's `which claude codex gemini openai`.
- **Step 4** → `shell.exec`: params `{script, env?, timeoutMs?}`, response
  `{stdout, stderr, exitCode}` — a real, generic `sh -c <script>` primitive
  (`agent-rpc-catalog-runtime.md:169`, confirmed at
  `agent/src/relay/fs-agent-extensions.ts:535-586`). BL-FLEET-04's Step 4
  checks (`git --version`, `node --version`, `df -P ~/.orca`, a port probe,
  `gh --version`) are all composable into one `shell.exec` script — the same
  mechanism [SOL-FLEET-03](./SOL-FLEET-03-health-monitoring-writer.md) uses
  for fleet-health metrics collection, reused here for a different purpose.

Both methods are reachable from **every** connection mode this service
supports (`relay-ssh` included, since backend-go's relay-ssh path deploys
`agent/out/agent.js` — the same Part A dispatcher — not the narrower Part B
`relay.js`, per `sshrelay`'s own package doc comment). This means Steps 3/4
work identically regardless of which of the three transport modes the
onboarding target uses — a meaningful simplification the BL spec (written
against the TS system's SSH-only onboarding flow) doesn't anticipate.

The existing `preflight.check` channel
(`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:565-573`)
is correctly left untouched and unrelated — it answers "can the browser's
own backend host run gh/glab", a different question from "can this specific
remote dev server be onboarded", per the bug report's own finding.

---

## Design — Step 2: persist and surface handshake facts

`domain.DevServer` gains the four fields SOL-FLEET-02 already introduces for
its own status tracking (`Platform, Arch, NodeVersion, AgentVersion string`)
— this solution is the second, matching write path into the same columns
(`migrations/0007_dev_server_status.up.sql`, see SOL-FLEET-02), since both
`EstablishConnection` (Step 1's SSH connect) and `BulkProvisionFleet` (Step 5)
receive a `devserveragent.HandshakeInfo` and both should persist it —
duplicating the columns per-solution would be a schema-ownership conflict,
so this solution depends on SOL-FLEET-02's migration rather than adding a
second one.

```go
// internal/usecase/establish_connection.go — one line added after the
// existing Health() check, before constructing Connection:
reachable, err := uc.agent.Health(ctx, devServer)
if err != nil || !reachable {
    return domain.Connection{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SSH_CONNECT_FAILED", "failed to establish SSH connection to target", err)
}
// NEW: Health() only returns a bool today (client.go:276-283) — extended to
// also return the HandshakeInfo it already computes internally (session
// already holds it post-handshake, see session.go's isHandshaked/attachTransport),
// so this call site can persist it without a second round trip.
if info, ok := uc.agent.LastHandshakeInfo(devServer.ID); ok {
    _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusHealthy, info, time.Now())
}
```

`DevServerAgentClient` gains `LastHandshakeInfo(devServerID string) (HandshakeInfo, bool)` —
a cheap in-memory lookup against the already-live `session` (no new RPC),
since `session.attachTransport` already receives `HandshakeInfo` at connect
time (`devserveragent/client.go:211`, `:234`) and simply wasn't retained
anywhere queryable before this solution.

`ResolveConnection`'s `DevServer` proto message, and `RegisterDevServerResponse`/
`EstablishConnection`'s `Connection` response, all surface the new fields
directly (no separate RPC needed — the data rides along on messages already
returned). `wscompat`'s `toDevServerView` (`channels.go:377-388`) stops
hardcoding `Platform`/`Arch`/`NodeVersion` to `nil` and maps the real proto
fields — closing the exact gap the file's own doc comment flags
(`channels.go:334-337`, "none of the frontend's ... platform/arch/nodeVersion
... fields exist server-side yet").

## Design — Step 3: `DetectDevServerAgents`

```go
// internal/usecase/detect_dev_server_agents.go
var defaultAgentProbes = []AgentProbe{
    {ID: "claude", Cmd: "claude"}, {ID: "codex", Cmd: "codex"},
    {ID: "gemini", Cmd: "gemini"}, {ID: "openai", Cmd: "openai"},
} // BL-FLEET-04 Step 3's exact four, BL-FLEET-04-dev-server-onboarding.md:29

type DetectDevServerAgents struct {
    devServers DevServerRepository
    agent      DevServerAgentClient
}

func (uc *DetectDevServerAgents) Execute(ctx context.Context, tenantID, devServerID string) (DetectedAgents, error) {
    ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
    if err != nil {
        return DetectedAgents{}, err
    }
    result, err := uc.agent.Exec(ctx, ds, "preflight.detectAgents", map[string]any{
        "commands": defaultAgentProbes,
    })
    if err != nil {
        return DetectedAgents{}, apperrors.New(apperrors.KindUnavailable, "INFRA_DETECT_AGENTS_FAILED", "failed to detect installed AI agents", err)
    }
    return decodeDetectedAgents(result), nil // {agents: []string, platform: string}
}
```

## Design — Step 4: `CheckDevServerPreflight`

```go
// internal/usecase/check_dev_server_preflight.go
const preflightScript = `
echo "GIT:$(git --version 2>/dev/null)"
echo "NODE:$(node --version 2>/dev/null)"
echo "DISK:$(df -P ~/.orca 2>/dev/null | tail -1 | awk '{print $4}')"
echo "GH:$(gh --version 2>/dev/null | head -1)"
node -e "require('net').createServer().listen(%d,'127.0.0.1',()=>{console.log('PORT:FREE');process.exit(0)}).on('error',()=>{console.log('PORT:BUSY');process.exit(0)})"
` // one shell.exec round trip for the whole check, per BL-FLEET-04 Step 4's list

type PreflightCheckResult struct {
    Git, Node CheckResult // {Installed bool; Version string; MeetsMin bool}
    Disk      DiskCheckResult // {FreeGB float64; MeetsMin bool}
    Port      PortCheckResult // {Port int32; Available bool}
    GH        CheckResult // installed-only, no version-min
}

func (uc *CheckDevServerPreflight) Execute(ctx context.Context, tenantID, devServerID string, probePort int32) (PreflightCheckResult, error) {
    ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
    if err != nil {
        return PreflightCheckResult{}, err
    }
    result, err := uc.agent.Exec(ctx, ds, "shell.exec", map[string]any{
        "script": fmt.Sprintf(preflightScript, probePort), "timeoutMs": 8000,
    })
    if err != nil {
        // A shell.exec failure is itself informative here (agent
        // unreachable) — surfaced as a typed error, not a synthesized
        // all-false result, per this codebase's "never fabricate a zero
        // value" convention (e.g. InspectProcessResult.Known, ports.go:181-186).
        return PreflightCheckResult{}, apperrors.New(apperrors.KindUnavailable, "INFRA_PREFLIGHT_FAILED", "failed to run remote preflight check", err)
    }
    return parsePreflightOutput(result["stdout"].(string)), nil // pure parsing, unit-tested independently — Git>=2.25, Node>=22 thresholds match SOL-FLEET-02's prereq.go
}
```

Per BL-FLEET-04's "Allow: proceed with warnings (non-critical failures)"
(`BL-FLEET-04-dev-server-onboarding.md:41`), `CheckDevServerPreflight`
never itself blocks — it returns the full `PreflightCheckResult` with
per-check pass/fail and lets the caller (the frontend wizard) decide
whether to proceed; there is no server-side hard-fail branch here, matching
how `EstablishConnection` is the only step in this flow that *does* hard-fail
(a truly unreachable target has nothing else to check).

## Design — Step 5 (cross-reference only)

Not re-designed here. [SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md)'s
"What this solution does instead of a true daemon" section applies
verbatim to onboarding's Step 5: the real launch+handshake stands in for
"Start Relay" + "Health Check" until `agent/` gains an HTTP daemon surface.

## Design — Step 6: field-gap closure

`RegisterDevServerRequest` gains `relay_port` (int32, `0` = "no daemon port
— foreground stdio session", honest placeholder until SOL-FLEET-02's daemon
work lands, never fabricated). `sshKeyPath` is **not** added as a raw
filesystem path field — same security-invariant conflict SOL-FLEET-01
flags for `identityFile` (`infra-fleet-service.md` §9, no raw key material
ever). The wizard's Step 1 SSH-key-path input must resolve to a
`vaultSshRole` before reaching `RegisterDevServer`, exactly like
SOL-FLEET-01's YAML import path — this is the same architectural
boundary showing up at a second call site, not a new decision.
`status` in `devServer.list`'s response stops being hardcoded to
`"disconnected"` (`channels.go:382`) and instead reflects
`domain.DevServer.Status` (SOL-FLEET-02's field), computed honestly instead
of synthesized.

## Design — Step 7 and the state machine (explicit non-decisions)

**Step 7** is not given a new backend entity. Once
[SOL-FLEET-01](./SOL-FLEET-01-fleet-inventory-import.md)'s
`ImportFleetInventory` and [SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md)'s
`BulkProvisionFleet` both exist, the multi-server checklist is exactly
`BulkProvisionFleet`'s response (`Outcomes []ProvisionOutcome`, one row per
imported server) rendered as a checklist client-side — no new RPC, no new
domain concept. This is a deliberate design choice, not an oversight:
BL-FLEET-04 Step 7 is a UI view over data both other solutions already
produce.

**No backend `WizardSession`/state-machine entity is added.** Per
`03-clean-architecture-guidelines.md`'s usecase-per-RPC convention (already
cited by every usecase in this file), each wizard step maps to one
independently-callable, idempotent RPC
(`EstablishConnection`/`DetectDevServerAgents`/`CheckDevServerPreflight`/
`RegisterDevServer`, plus `BulkProvisionFleet` for the multi-server case) —
sequencing them into `IDLE → CONNECTING → ... → DONE` is presentation-layer
state the frontend wizard component already owns (the same posture BUG-FLEET-04's
own finding takes: "each step above is either an independent RPC or
entirely absent; no backend-go entity tracks onboarding progress" is treated
here as the *correct* end state for the RPC steps, not a gap to fill with a
new persisted state machine). Flagged explicitly, matching
`02-microservices-decomposition.md`'s "What's deliberately not a separate
service" framing for a deliberate non-implementation — worth confirming
against product intent if a resumable, multi-device onboarding session
turns out to be a real requirement later.

---

## Test plan

- `domain/dev_server_test.go` — `NewDevServer` unaffected; new fields
  default to zero values, no invariant added (shared coverage with
  SOL-FLEET-02).
- `usecase/detect_dev_server_agents_test.go` — fake `DevServerAgentClient.Exec`
  asserts called with `"preflight.detectAgents"` and the exact 4-probe
  `commands` list; decodes a fixture `{agents:["claude"],platform:"linux"}`
  response correctly; an `Exec` error surfaces as `INFRA_DETECT_AGENTS_FAILED`,
  not a fabricated empty list.
- `usecase/check_dev_server_preflight_test.go` — fake `Exec` returns a
  fixture `stdout` block; asserts `Git.MeetsMin=true` for `2.39.2` and
  `false` for `2.20.0` (matches Node>=22/Git>=2.25 thresholds, shared test
  vectors with SOL-FLEET-02's `prereq_test.go`); `Port.Available` parses
  both `PORT:FREE`/`PORT:BUSY`; malformed `stdout` degrades every field to
  `Installed=false`/`MeetsMin=false`, never panics.
- `usecase/establish_connection_test.go` — extended: fake `DevServerAgentClient`
  with `LastHandshakeInfo` returning a fixture `HandshakeInfo` asserts
  `UpdateProvisionResult` is called with the right platform/arch/version
  fields after a successful connect; `LastHandshakeInfo`'s `ok=false` branch
  (defensive — should not happen post-`Health()==true`, but tested) skips
  the persist call without erroring the whole `Execute`.
- `wscompat/channels_test.go` — `toDevServerView` maps real (non-nil)
  platform/arch/nodeVersion/status fields when present, falls back to the
  existing honest-nil placeholders when a `DevServer` predates this
  migration (zero-value fields).
- Contract test: full wizard happy path against fakes — `EstablishConnection`
  → `DetectDevServerAgents` → `CheckDevServerPreflight` → (Step 5, unchanged)
  → `RegisterDevServer` — each call succeeds independently and in isolation
  (no hidden ordering dependency enforced server-side), confirming the
  "no state machine needed" design decision actually holds up under test.

## References

- `docs/logic/fleet/BL-FLEET-04-dev-server-onboarding.md` — 7-step wizard,
  state machine
- `specs/backend-go/bugs/logic-v1/BUG-FLEET-04-dev-server-onboarding-partial.md`
- `specs/backend-go/tdd/services/infra-fleet-service.md:55-58,65,69,142-166`
  (§2 bounded context, §4 domain model)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` —
  usecase-per-RPC convention this solution's "no state machine entity"
  decision cites
- `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go:31-77`
  (Step 1, real, extended here)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:117-172`
  (handshake capture, currently discarded — the exact gap this solution closes)
- `backend-go/services/infra-fleet-service/internal/domain/connection.go:19-33`
  (no platform fields today)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:211,234,276-283`
  (`HandshakeInfo` already captured at attach time, not retained queryably)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:334-337,377-388,565-573`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:112-134` (`DevServer`/
  `RegisterDevServerRequest` messages extended here)
- `specs/agent/api/agent-rpc-catalog-runtime.md:169,191-192` (`shell.exec`,
  `preflight.detectAgents` — confirmed real via direct `agent/src/relay/
  agent-rpc-dispatch.ts`/`preflight-handler.ts`/`fs-agent-extensions.ts`
  inspection) — no `agent/` change required for Steps 3/4
- [SOL-FLEET-01](./SOL-FLEET-01-fleet-inventory-import.md) — `identityFile`/
  Vault-role security-invariant precedent this solution's Step 6/1 field
  decisions reuse; Step 7's data source
- [SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md) — shared
  `Status`/platform-field migration and daemon-model gap this solution
  depends on / cross-references
- [SOL-FLEET-03](./SOL-FLEET-03-health-monitoring-writer.md) — the sibling
  use of `shell.exec` for metrics collection, same mechanism this solution
  reuses for preflight checks
