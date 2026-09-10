# TASK-TM-04-02: Wire OSC 133 bootstrap into relay-mode `windowsShellArgs`, gated by opt-in flag

**From Solution:** SOL-TM-04
**Priority:** P0
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/pty-shell-launch.ts`
**Depends on:** TASK-TM-04-01 (bootstrap export)
**Status:** `[x]` DONE — windowsShellArgs + getRelayShellLaunchConfig now accept shellIntegration (default-false, opt-in); `npx vitest run src/relay/pty-shell-launch.test.ts` — 16/16 pass (5 new cases covering true/false/omitted, cmd/wsl unaffected, POSIX unaffected).

---

## Context

`windowsShellArgs` currently returns bare `['-NoLogo']` for
`powershell.exe`/`pwsh.exe`/`powershell`/`pwsh` with no bootstrap at all —
this is the real gap `BUG-TM-04` describes for the relay-mode spawn path
(the Electron-main *local*-PTY path already has this via
`windows-shell-args.ts`). BR-TM-13 makes injection opt-in, so the new
`shellIntegration` option must default to today's unchanged behavior when
omitted/false. `cmd.exe`/`wsl.exe` branches and POSIX shells (whose OSC 133
emission via `ensureOverlayRestoreWrappers`' `bashRc` template is already
unconditional and live in production for a different purpose — agent
busy/idle detection) are untouched by this task.

## Changes to make

In `agent/src/relay/pty-shell-launch.ts`:

1. Add the import:

```typescript
import { encodePowerShellCommand, getPowerShellOsc133Bootstrap } from './pty-osc133-bootstrap'
```

2. Replace `windowsShellArgs`:

```typescript
function windowsShellArgs(
  shellName: string,
  options: { terminalWindowsWslDistro?: string | null; shellIntegration?: boolean } = {}
): string[] | null {
  if (shellName === 'powershell.exe' || shellName === 'powershell' ||
      shellName === 'pwsh.exe' || shellName === 'pwsh') {
    if (!options.shellIntegration) {
      return ['-NoLogo']
    }
    // Why: -NoExit keeps the bootstrapped prompt/readline functions active
    // for the session's lifetime — PowerShell never emits OSC 133 natively
    // (BR-TM-14), so the wrapper must stay loaded, not run once and exit.
    const encoded = encodePowerShellCommand(getPowerShellOsc133Bootstrap())
    return ['-NoLogo', '-NoExit', '-EncodedCommand', encoded]
  }
  if (shellName === 'cmd.exe' || shellName === 'cmd') {
    return []
  }
  if (shellName === 'wsl.exe' || shellName === 'wsl') {
    const distro = options.terminalWindowsWslDistro?.trim()
    return distro ? ['-d', distro] : []
  }
  return null
}
```

3. Thread the option through `getRelayShellLaunchConfig`'s signature and its
   `windowsShellArgs` call:

```typescript
export function getRelayShellLaunchConfig(
  shellPath: string,
  env: Record<string, string>,
  platform: NodeJS.Platform = process.platform,
  options: {
    emitReadyMarker?: boolean
    terminalWindowsWslDistro?: string | null
    shellIntegration?: boolean
  } = {}
): RelayShellLaunchConfig {
  const shellName = shellBasename(shellPath)
  const emitReadyMarker = options.emitReadyMarker === true
  if (platform === 'win32') {
    // Why: pwsh also exists on POSIX remotes; Windows-specific shell args must
    // only apply when the relay itself is running on native Windows.
    return {
      args:
        windowsShellArgs(shellName, {
          terminalWindowsWslDistro: options.terminalWindowsWslDistro,
          shellIntegration: options.shellIntegration
        }) ?? [],
      env: {}
    }
  }
  // ... rest of the function (POSIX branches) unchanged ...
```

## Verify

```bash
cd /opt/repos/orca/agent
npx tsc --noEmit -p tsconfig.json
```

Extend `agent/src/relay/pty-shell-launch.test.ts`:
- `shellIntegration: false` (or omitted) keeps today's bare `['-NoLogo']`
  for `powershell.exe`/`pwsh.exe`/`powershell`/`pwsh` on `platform: 'win32'`
  (no behavior change for existing callers)
- `shellIntegration: true` returns
  `['-NoLogo', '-NoExit', '-EncodedCommand', <encoded>]` where
  `Buffer.from(encoded, 'base64').toString('utf16le')` contains the OSC 133
  bootstrap markers (`]133;A`, `]133;C`, etc.)
- `cmd.exe`/`wsl.exe` branches produce identical output regardless of
  `shellIntegration`
- POSIX (`darwin`/`linux`) platform argument shapes are unaffected by the
  new option

```bash
npx vitest run src/relay/pty-shell-launch.test.ts
```

Expected: clean typecheck, all cases pass.
