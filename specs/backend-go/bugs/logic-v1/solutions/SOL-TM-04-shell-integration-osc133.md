# SOL-TM-04: PowerShell OSC 133 bootstrap injection + opt-in shell-integration flag

**Resolves:** [BUG-TM-04](../BUG-TM-04-shell-integration-osc133-not-implemented.md)
**Service:** Dev Server Agent (`agent/`) — primary fix — + `infra-fleet-service` + `api-gateway` (coordination-layer plumbing only)
**Affected files (proposed):**
- `agent/src/relay/pty-osc133-bootstrap.ts` (new — ports `backend/src/main/powershell-osc133-bootstrap.ts`)
- `agent/src/relay/pty-shell-launch.ts` (`windowsShellArgs` — inject bootstrap for PowerShell)
- `agent/src/relay/pty-handler.ts` (`spawn` — thread `shellIntegration` param through to `getRelayShellLaunchConfig`)
- `agent/src/relay/agent-rpc-dispatch.ts` (`pty.create` doc comment — new param)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`SpawnTerminalSessionRequest.shell_integration`)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`, `ports.go` (`SpawnPtyInput.ShellIntegration`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go` (`terminalCreateArgs.ShellIntegration`)
- Test files: `agent/src/relay/pty-shell-launch.test.ts`, `agent/src/relay/pty-handler.test.ts`, `infra-fleet-service/internal/usecase/spawn_terminal_session_test.go`, `wscompat/channels_terminal_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Where PTY bytes actually flow — this is mostly an `agent/` fix, not a backend-go one

`infra-fleet-service.md` §2's bounded-context table draws the line
explicitly: "PTY byte I/O (the actual terminal data stream) | No — routes
the request to the right connection, **does not touch the bytes** | Yes
(`node-pty` **on the target host**)" (`infra-fleet-service.md:66`), and §1
reiterates it does not "run a shell command... or move PTY bytes itself"
(`infra-fleet-service.md:37`). `BUG-TM-04`'s own findings confirm this
holds today: `terminal.output`/`terminal.multiplex`'s `Output` opcode both
forward `PtyOutput.Data` verbatim, with no escape-sequence awareness
(`channels_terminal.go:258-259`, `channels_terminal_multiplex.go:222-230`).
OSC 133 is entirely a property of the byte stream a shell emits — where
that stream first exists is `node-pty` on the Dev Server Agent
(`agent/src/relay/pty-handler.ts`), never backend-go. Both halves of
BL-TM-04's actual gap — BR-TM-14's bootstrap *injection* (which needs to
write/pass a script to the process being spawned) and any *detection* of
the sequences it produces — belong on that side of the boundary by
construction, not by choice.

### The mechanism already exists — twice — just not in the path backend-go's agent uses today

This is the load-bearing finding for this solution: OSC 133 bootstrap
injection and detection are **not net-new engineering** here. Two working
reference implementations already exist in this repository:

1. **PowerShell bootstrap** — `backend/src/main/powershell-osc133-bootstrap.ts`
   is a complete, working `-EncodedCommand` PowerShell bootstrap script
   that wraps `$function:prompt`/`PSConsoleHostReadLine` to emit OSC
   133;A/B/C/D around every command (`powershell-osc133-bootstrap.ts:31-68`),
   and `backend/src/main/providers/windows-shell-args.ts:163-175` wires it
   into the actual `pty.spawn` args for `powershell.exe`/`pwsh.exe` via
   `getPowerShellEncodedCommand`. **Every dependency this script needs
   already exists in `agent/src`**: `getPowerShellOmpShellWrapper`
   (`agent/src/main/pty/omp-shell-wrapper.ts:74`) and
   `encodePowerShellCommand` (`agent/src/shared/powershell-command-encoding.ts`,
   confirmed present) are both already ported/shared. Porting the bootstrap
   script itself is close to a direct copy, not new design.
2. **Bash OSC 133 A/C/D emission** — `agent/src/relay/pty-shell-launch.ts:156-234`
   (`ensureOverlayRestoreWrappers`'s `bashRc` template) **already emits
   real OSC 133;A/C/D sequences today**, with the file's own comment
   explaining why: "SSH bash sessions need the same command lifecycle
   markers as local bash so agent rows stop showing 'working' when the
   foreground command exits" (`pty-shell-launch.ts:154-155`). This is
   currently wired for a *different* purpose — internal AI-coding-agent
   busy/idle detection (feeding `GetTerminalAgentStatus`'s heuristic,
   `infra-fleet-service/internal/usecase/ports.go:173-179`) — not
   BL-TM-04's user-facing exit-code/command-navigation feature. It proves
   the emission mechanism for POSIX shells is already correct and already
   deployed; there is a parallel, unwired chunk-boundary-safe *parser* for
   the same sequences at `agent/src/shared/terminal-osc133-command-finished.ts`
   (its own doc comment: "main parses every local-daemon/SSH PTY byte
   exactly once and emits what it observed", `:1-9`), confirmed **not**
   currently imported anywhere under `agent/src/relay/*.ts` — i.e., shared,
   ported, tested code sitting unused in the path backend-go's
   `DevServerAgentClient` actually talks to.

`BUG-TM-04`'s "zero OSC 133 awareness anywhere in backend-go" finding is
accurate for backend-go itself, but the wider system does not need
`backend-go` to gain byte-parsing — it needs `agent/` to (a) close the one
real gap (PowerShell bootstrap wiring into the *relay* spawn path,
`pty-shell-launch.ts`'s `windowsShellArgs`, as opposed to the *local*
Electron-main spawn path in `backend/src/main/providers/windows-shell-args.ts`
that already has it) and (b) optionally wire the existing-but-unused
scanner. This is a materially smaller fix than "implement OSC 133 support"
reads as.

### The interactive UI feature (exit codes, jump-between-commands) needs no backend-go RPC at all

Reading the real protocol's `Metadata` opcode (`TerminalStreamOpcodeMetadata`,
value 12, defined but unused in `channels_terminal_multiplex.go`) against
its one real usage in the old system
(`backend/src/main/runtime/rpc/methods/terminal.ts:2086-2098`) shows it
only ever carries `{cwd}` — there is **no dedicated wire frame for
command-boundary metadata anywhere in `terminal-stream-protocol.ts`**.
Since raw PTY bytes already flow to the client unmodified end-to-end
(`BUG-TM-04`'s own citations, `channels_terminal.go:258-274`,
`channels_terminal_multiplex.go:222-240`), the client already receives
every OSC 133 byte a shell emits. Rendering ✅/❌/⏱ and jumping between
commands is therefore natively a **frontend xterm.js concern** — the same
way every other terminal emulator implements shell integration (VS Code,
iTerm2, etc. all parse OSC 133 in the terminal renderer itself, via
xterm.js's own `Terminal.parser.registerOscHandler(133, ...)` or
equivalent), not something a server needs to pre-parse for. Once
PowerShell actually emits the bytes (this solution's fix), the client-side
feature is unblocked with **zero further backend-go changes** — flagged
explicitly because `BUG-TM-04`'s own framing ("backend-go has zero
awareness of OSC 133 at all") could be misread as implying backend-go must
gain parsing to fix the user-facing feature. It does not.

### BR-TM-13 (opt-in) — backend-go's actual, narrow contribution

The one piece of BL-TM-04 that legitimately belongs at the coordination
layer is the **opt-in flag itself**: whether shell-integration bootstrap
injection happens at all for a given session is a per-session choice made
at spawn time, and `SpawnTerminalSessionRequest` is exactly the message
this decision already rides through today for every other spawn-time
choice (`shell`, `cwd`, `cols`/`rows` —
`infrafleet.proto:297-303`). This is a plain pass-through value,
structurally identical to how `Shell` itself is already forwarded
unexamined from `SpawnTerminalSessionRequest` through `SpawnPtyInput` to
the agent's `pty.create` params
(`infra-fleet-service/internal/usecase/spawn_terminal_session.go:69`) —
backend-go adds one boolean field to that existing pass-through, nothing
more. It does **not** gate the flag on shell-content inspection or decide
*how* injection happens — that decision (which script, how to invoke it)
stays entirely in `agent/`, preserving the "coordination decides whether,
execution decides how" split `infra-fleet-service.md` §2's table draws for
every other capability.

Note this is a deliberate divergence from `windows-shell-args.ts`'s own
precedent, where the old system injects the PowerShell bootstrap
**unconditionally** for every `powershell.exe`/`pwsh.exe` spawn, with no
opt-in check at that call site at all (`windows-shell-args.ts:163-175`).
BL-TM-04's literal BR-TM-13 text ("opt-in, không bắt buộc" — opt-in, not
mandatory) reads as a real per-session control, so this solution makes it
one rather than reproducing the old unconditional-injection behavior —
flagged explicitly as a chosen behavior change, not an oversight. The
bootstrap script itself is harmless to run even for a session that never
looks at the sequences (a non-participating client simply never renders
the invisible OSC bytes), so defaulting `shell_integration` to `false` for
existing callers that don't set it is a safe, additive migration.

### BR-TM-15 (strip OSC before clipboard copy)

Confirmed client-only per `BUG-TM-04`'s own finding — no backend-go or
`agent/` design needed; carried forward unchanged.

---

## Design — `agent/` (primary fix)

### 1. Port the PowerShell OSC 133 bootstrap

```typescript
// agent/src/relay/pty-osc133-bootstrap.ts
// Ported from backend/src/main/powershell-osc133-bootstrap.ts — see
// SOL-TM-04's rationale for why this is a near-direct port (both
// dependencies it needs, getPowerShellOmpShellWrapper and
// encodePowerShellCommand, already exist in agent/src).
import { getPowerShellOmpShellWrapper } from '../main/pty/omp-shell-wrapper'
export { encodePowerShellCommand } from '../shared/powershell-command-encoding'

const POWERSHELL_OSC133_BOOTSTRAP = `# Orca OSC 133 shell integration for PowerShell.
...` // identical content to backend/src/main/powershell-osc133-bootstrap.ts:4-69

export function getPowerShellOsc133Bootstrap(): string {
  return POWERSHELL_OSC133_BOOTSTRAP
}

export function isPowerShellExecutableName(shellName: string): boolean {
  const normalized = shellName.toLowerCase()
  return (
    normalized === 'pwsh' || normalized === 'pwsh.exe' ||
    normalized === 'powershell' || normalized === 'powershell.exe'
  )
}
```

### 2. Wire it into the relay-mode spawn path, gated by the new opt-in flag

`windowsShellArgs` (`pty-shell-launch.ts:28-46`) currently returns bare
`['-NoLogo']` for PowerShell with no bootstrap at all — this is the actual
gap `BUG-TM-04` describes for the path backend-go's agent uses (the
Electron-main *local*-PTY path already has this, per
`windows-shell-args.ts`, but relay mode does not):

```typescript
// pty-shell-launch.ts, windowsShellArgs — extended
function windowsShellArgs(
  shellName: string,
  options: { terminalWindowsWslDistro?: string | null; shellIntegration?: boolean } = {}
): string[] | null {
  if (shellName === 'powershell.exe' || shellName === 'powershell' ||
      shellName === 'pwsh.exe' || shellName === 'pwsh') {
    if (!options.shellIntegration) {
      return ['-NoLogo']
    }
    const encoded = encodePowerShellCommand(getPowerShellOsc133Bootstrap())
    // Why: -NoExit keeps the bootstrapped prompt/readline functions active
    // for the session's lifetime — matches windows-shell-args.ts's own
    // rationale (BR-TM-14: PowerShell needs a prompt/readline wrapper since
    // it never emits OSC 133 natively).
    return ['-NoLogo', '-NoExit', '-EncodedCommand', encoded]
  }
  // ... cmd.exe / wsl.exe branches unchanged ...
}
```

`getRelayShellLaunchConfig`'s signature gains `shellIntegration?: boolean`
in its `options` parameter, threaded straight into `windowsShellArgs` — no
other branch of this function changes; POSIX shells' OSC 133 emission is
already unconditional today via `ensureOverlayRestoreWrappers` (see
rationale above) and this solution does not change that behavior, only
adds the PowerShell parity fix. (A follow-up could gate the POSIX path on
the same flag for consistency with BR-TM-13's literal "opt-in" wording;
flagged as an intentionally deferred scope decision, not a gap this
solution needs to close — the POSIX emission is already live in
production today via the "agent busy/idle" use case, so gating it now is a
behavior change with its own blast radius, better done as a separate,
deliberate pass.)

### 3. Thread the flag from `pty.create`

```typescript
// pty-handler.ts, spawn() — new param, same shape as shellOverride/command
const shellIntegration = params.shellIntegration === true
...
const shellLaunch = getRelayShellLaunchConfig(shell, spawnEnv, process.platform, {
  terminalWindowsWslDistro,
  emitReadyMarker: shouldEmitShellReadyMarker,
  shellIntegration
})
```

`agent-rpc-dispatch.ts`'s `pty.create` doc comment
(`agent-rpc-dispatch.ts:1473`) gains `shellIntegration?` to its documented
params list.

---

## Design — `infra-fleet-service` + `api-gateway` (pass-through only)

### Proto

```protobuf
message SpawnTerminalSessionRequest {
  string connection_id = 1;
  string cwd = 2;
  string shell = 3;
  int32  cols = 4;
  int32  rows = 5;
  // BR-TM-13 — opt-in shell-integration bootstrap (OSC 133). Forwarded
  // unexamined to the agent's pty.create; see SOL-TM-04. Defaults to
  // false — existing callers that don't set it see no behavior change.
  bool   shell_integration = 6;
}
```

### `usecase.SpawnTerminalSession` — one new pass-through field

```go
// SpawnTerminalSessionInput — extended
type SpawnTerminalSessionInput struct {
	ConnectionID     string
	Cwd              string
	Shell            string
	Cols, Rows       int32
	ShellIntegration bool // BR-TM-13 — forwarded to SpawnPtyInput, never inspected here
}
```

```go
// spawn_terminal_session.go's Execute — one line added to the existing
// uc.agent.SpawnPty call:
result, err := uc.agent.SpawnPty(ctx, devServer, SpawnPtyInput{
	Cwd: in.Cwd, Shell: in.Shell, Cols: in.Cols, Rows: in.Rows,
	ShellIntegration: in.ShellIntegration,
})
```

`SpawnPtyInput` (`ports.go:190-195`) gains `ShellIntegration bool`;
`adapter/devserveragent`'s `SpawnPty` implementation adds
`"shellIntegration": in.ShellIntegration` to the `pty.create` JSON-RPC
params map it already builds — no new method needed on
`DevServerAgentClient`, this rides the existing `SpawnPty` typed wrapper.

### `wscompat`

```go
// channels_terminal.go — terminalCreateArgs extended
type terminalCreateArgs struct {
	ConnectionID     string `json:"connectionId"`
	Cwd              string `json:"cwd"`
	Shell            string `json:"shell"`
	Cols             int32  `json:"cols"`
	Rows             int32  `json:"rows"`
	ShellIntegration bool   `json:"shellIntegration"` // BR-TM-13
}
```

`registerTerminalCreateChannel`'s `SpawnTerminalSessionRequest` call gains
`ShellIntegration: in.ShellIntegration` — the one line this feature adds
to `channels_terminal.go`.

---

## Explicitly out of scope for this solution

- **Command-boundary UI (exit code badges, jump-between-commands)** — no
  backend-go or `agent/` change needed; see rationale above. This is a
  `frontend/` (xterm.js `registerOscHandler`) feature, tracked separately
  if it isn't already implemented client-side.
- **Wiring `terminal-osc133-command-finished.ts`'s parser into a
  queryable/pushable backend RPC** (e.g., "did the last command in this
  still-open shell succeed", which `BUG-TM-04`'s own text flags as
  unanswerable today) — genuinely useful for a non-UI caller (e.g. a
  future workflow-service step polling shell command status), and the
  scanner already exists and is already proven in production for the
  agent-busy/idle use case, so wiring it is low-risk. Not required to
  close BL-TM-04's actual business rules (BR-TM-13/14/15 make no mention
  of a query RPC), so left as a flagged, ungated **future extension**
  rather than folded into this fix's scope — doing so would conflate two
  independently-motivated features (interactive command tracking vs.
  programmatic status query) in one change.

---

## Test plan

- `agent/src/relay/pty-shell-launch.test.ts` — `windowsShellArgs`/
  `getRelayShellLaunchConfig`: `shellIntegration: false` (or omitted)
  keeps today's bare `['-NoLogo']` for PowerShell (no behavior change for
  existing callers); `shellIntegration: true` returns
  `['-NoLogo', '-NoExit', '-EncodedCommand', <encoded>]` where the decoded
  payload contains the OSC 133 bootstrap; cmd.exe/wsl.exe branches
  unaffected by the new option.
- `agent/src/relay/pty-handler.test.ts` — `spawn()` with
  `params.shellIntegration: true` against a PowerShell `shell` produces a
  `pty.spawn` call whose args include `-EncodedCommand`; omitted/false
  param produces the unchanged args.
- Round-trip test against the existing
  `agent/src/shared/__tests__/terminal-osc133-command-finished.test.ts`
  fixtures: feed the bootstrap's OSC 133;A/B/C/D output (once decoded)
  through `createOsc133CommandFinishedScanner` and confirm it fires
  `onCommandStarted`/`onCommandFinished` with the right exit code — proves
  the ported bootstrap and the already-existing parser agree on wire
  format (regression guard against silent drift between the two ported
  pieces).
- `infra-fleet-service/internal/usecase/spawn_terminal_session_test.go` —
  fake `DevServerAgentClient`: asserts `ShellIntegration` on the input
  reaches `SpawnPtyInput.ShellIntegration` unmodified; defaults to `false`
  when unset.
- `api-gateway/internal/adapter/wscompat/channels_terminal_test.go` —
  `terminal.create` with `shellIntegration: true` in args produces a
  `SpawnTerminalSessionRequest` with `ShellIntegration: true`.
- Manual/integration smoke: spawn a `pwsh.exe` relay PTY with
  `shellIntegration: true`, run a command, confirm raw OSC 133;A/B/C/D
  bytes appear in the `terminal.output`/`Output` frames the client
  receives (proves end-to-end byte flow without backend-go interpreting
  them, per the architecture boundary this solution preserves).

## References

- `docs/logic/terminal-management/BL-TM-04-shell-integration.md` — BR-TM-13/14/15
- `specs/backend-go/tdd/services/infra-fleet-service.md:53-76` (§2 bounded-context table — "does not touch the bytes"), `:140-166` (§4 domain model, `TerminalSession` scope), `:173-179` (`AgentStatus`'s existing agent-computed-metadata-relayed-not-parsed precedent)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:59-104` (usecase/adapter layering — why the flag is a pass-through, not inspected logic, at the infra-fleet-service layer)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:22-36,48-94` (existing `Shell` pass-through precedent this solution's `ShellIntegration` field follows)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:121-195` (`DevServerAgentClient`/`SpawnPtyInput` — extension point)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:180-235,258-274` (`terminal.create` args, raw-byte-passthrough confirmation)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:222-240` (binary-opcode raw passthrough, `TerminalStreamOpcodeMetadata` unused-today confirmation)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:297-303` (`SpawnTerminalSessionRequest` — extension point)
- `backend/src/main/powershell-osc133-bootstrap.ts:1-84` (bootstrap script to port, verbatim reference)
- `backend/src/main/providers/windows-shell-args.ts:70-97,163-175` (`getPowerShellEncodedCommand`/wiring precedent — old system's *local*-PTY path, unconditional injection)
- `agent/src/relay/pty-shell-launch.ts:28-46,154-234,263-310` (`windowsShellArgs`/`getRelayShellLaunchConfig` — extension point; existing bash OSC 133 emission proving the mechanism already works for POSIX shells)
- `agent/src/relay/pty-handler.ts:633-742` (`spawn()` — where `shell`/`shellLaunch.args` reach `pty.spawn`)
- `agent/src/relay/agent-rpc-dispatch.ts:1471-1489` (`pty.create` dispatch case and its documented params)
- `agent/src/main/pty/omp-shell-wrapper.ts:74` (`getPowerShellOmpShellWrapper` — already-available dependency)
- `agent/src/shared/powershell-command-encoding.ts` (`encodePowerShellCommand` — already-available dependency)
- `agent/src/shared/terminal-osc133-command-finished.ts:1-108` (existing, currently-unwired POSIX OSC 133;C/D scanner — reusable as-is for the flagged future extension)
- `agent/src/shared/terminal-side-effect-facts.ts:18-22` (`command-finished` fact shape the scanner already produces, for the flagged future extension)
