# TASK-TM-04-03: Thread `shellIntegration` from `pty.create` into `spawn()`

**From Solution:** SOL-TM-04
**Priority:** P1
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/pty-handler.ts`
**Depends on:** TASK-TM-04-02 (`getRelayShellLaunchConfig` accepts the option)
**Status:** `[ ]` TODO

---

## Context

`pty.create`'s RPC params reach `PtyHandler.spawn()` as an untyped
`Record<string, unknown>` (same pattern every other optional param on this
method already uses — `params.command`, `params.terminalWindowsWslDistro`,
etc.). This task reads `params.shellIntegration`, defaulting to `false` so
existing callers that don't set it see no behavior change, and passes it
into the `getRelayShellLaunchConfig` call added in TASK-TM-04-02.

## Changes to make

In `agent/src/relay/pty-handler.ts`, inside `private async spawn(...)`
(around the existing `shouldEmitShellReadyMarker` computation), add:

```typescript
// BR-TM-13 — opt-in shell-integration bootstrap (OSC 133). Defaults to
// false: existing callers that don't set it see no behavior change.
const shellIntegration = params.shellIntegration === true
```

Then extend the existing `getRelayShellLaunchConfig` call:

```typescript
const shellLaunch = getRelayShellLaunchConfig(shell, spawnEnv, process.platform, {
  terminalWindowsWslDistro,
  emitReadyMarker: shouldEmitShellReadyMarker,
  shellIntegration
})
```

In `agent/src/relay/agent-rpc-dispatch.ts`, update the `pty.create` case's
doc comment (around line 1473) to document the new param:

```typescript
// ── v5.0: pty.create ─────────────────────────────────────────────────────
// TM-001/TM-006: Create a PTY session in agent mode.
// Params: { cwd, cols?, rows?, env?, shellOverride?, shellIntegration? }
// shellIntegration (BR-TM-13, SOL-TM-04): opt-in PowerShell OSC 133
// bootstrap injection — default false, no effect on non-PowerShell shells.
// Returns: { id, cols, rows, cwd, shell }
```

## Verify

```bash
cd /opt/repos/orca/agent
npx tsc --noEmit -p tsconfig.json
```

Extend `agent/src/relay/pty-handler.test.ts`:
- `spawn()` called with `params.shellIntegration: true` against a
  PowerShell `shell` produces a `pty.spawn` call whose args include
  `-EncodedCommand`
- omitted/`false` `params.shellIntegration` produces the unchanged
  `['-NoLogo']` args (regression guard — confirms the default truly
  preserves today's behavior)

```bash
npx vitest run src/relay/pty-handler.test.ts
```

Expected: clean typecheck, both cases pass.
