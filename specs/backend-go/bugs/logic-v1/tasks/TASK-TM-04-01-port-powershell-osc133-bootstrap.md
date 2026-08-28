# TASK-TM-04-01: Port PowerShell OSC 133 bootstrap script into `agent/`

**From Solution:** SOL-TM-04
**Priority:** P0 — every later task in this set depends on this export
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/pty-osc133-bootstrap.ts`
**Depends on:** none
**Status:** `[x]` DONE — agent/src/relay/pty-osc133-bootstrap.ts ported verbatim + smoke test added; `npx vitest run src/relay/pty-osc133-bootstrap.test.ts` — 3/3 pass. `npx tsc --noEmit` has pre-existing, unrelated TS6307 "file not in project" noise across this tsconfig (its `include` list already excludes files pre-existing code imports, e.g. `pty-shell-launch.ts`'s own `omp-shell-wrapper` import) — confirmed present before this change and not introduced by it.

---

## Context

`backend/src/main/powershell-osc133-bootstrap.ts` (Electron-main's *local*-PTY
path) already has a complete, working PowerShell bootstrap script that wraps
`$function:prompt`/`PSConsoleHostReadLine` to emit OSC 133;A/B/C/D around
every command. Both of its dependencies
(`getPowerShellOmpShellWrapper` at `agent/src/main/pty/omp-shell-wrapper.ts:74`,
`encodePowerShellCommand` at `agent/src/shared/powershell-command-encoding.ts`)
already exist in `agent/src` — this is a near-direct port, not new design.
`agent/`'s relay-mode spawn path (`pty-shell-launch.ts`) has no equivalent
today, which is the actual gap TASK-TM-04-02 closes.

## Changes to make

Create `agent/src/relay/pty-osc133-bootstrap.ts` — copy the bootstrap
content verbatim from `backend/src/main/powershell-osc133-bootstrap.ts`,
retargeting its one import path:

```typescript
// Ported from backend/src/main/powershell-osc133-bootstrap.ts (Electron
// main's local-PTY path) for the relay-mode spawn path — see
// SOL-TM-04's rationale for why this is a near-direct port: both
// dependencies it needs, getPowerShellOmpShellWrapper and
// encodePowerShellCommand, already exist in agent/src.
import { getPowerShellOmpShellWrapper } from '../main/pty/omp-shell-wrapper'
export { encodePowerShellCommand } from '../shared/powershell-command-encoding'

const POWERSHELL_OSC133_BOOTSTRAP = `# Orca OSC 133 shell integration for PowerShell.
if ((Test-Path variable:global:__OrcaOsc133State) -and
    $null -ne $Global:__OrcaOsc133State.OriginalPrompt) {
    return
}

if ($ExecutionContext.SessionState.LanguageMode -ne "FullLanguage") {
    return
}

# Profiles have already loaded normally by the time -EncodedCommand runs.
# Wrap the user's final prompt/readline state; do not source profiles here.

# Preserve Windows CJK output by keeping ConPTY on UTF-8 without bypassing
# profile loading or execution-policy checks.
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new()
    $OutputEncoding = [Console]::OutputEncoding
} catch { Write-Error $_ -ErrorAction Continue }

# Profiles can re-export user defaults after Orca's spawn env is set.
if ($env:ORCA_OPENCODE_CONFIG_DIR) { $env:OPENCODE_CONFIG_DIR = $env:ORCA_OPENCODE_CONFIG_DIR }
if ($env:ORCA_MIMOCODE_HOME) { $env:MIMOCODE_HOME = $env:ORCA_MIMOCODE_HOME }
${getPowerShellOmpShellWrapper()}
if ($env:ORCA_CODEX_HOME) { $env:CODEX_HOME = $env:ORCA_CODEX_HOME }

$Global:__OrcaOsc133State = @{
    OriginalPrompt = $function:prompt
    OriginalReadLine = $function:PSConsoleHostReadLine
    HasSeenPrompt = $false
    HasPSReadLine = $null -ne (Get-Module -Name PSReadLine)
    Esc = [char]27
    Bel = [char]7
}

function Global:prompt {
    # Capture FIRST; any other expression can clobber PowerShell's success bit.
    $fakeExitCode = [int](!$global:?)
    Set-StrictMode -Off
    $result = ""

    # Emit D from prompt, not readline state. Some profile setups bypass
    # PSConsoleHostReadLine; the consumer only needs completion.
    if ($Global:__OrcaOsc133State.HasSeenPrompt) {
        $result += "$($Global:__OrcaOsc133State.Esc)]133;D;$fakeExitCode$($Global:__OrcaOsc133State.Bel)"
    }
    $Global:__OrcaOsc133State.HasSeenPrompt = $true

    $result += "$($Global:__OrcaOsc133State.Esc)]133;A$($Global:__OrcaOsc133State.Bel)"
    # Preserve the previous success/failure value for prompts that inspect it.
    if ($fakeExitCode -ne 0) { Write-Error "failure" -ea ignore }
    $result += $Global:__OrcaOsc133State.OriginalPrompt.Invoke()
    $result += "$($Global:__OrcaOsc133State.Esc)]133;B$($Global:__OrcaOsc133State.Bel)"
    $result
}

if ($Global:__OrcaOsc133State.HasPSReadLine -and
    $null -ne $Global:__OrcaOsc133State.OriginalReadLine) {
    function Global:PSConsoleHostReadLine {
        $commandLine = $Global:__OrcaOsc133State.OriginalReadLine.Invoke()
        [Console]::Write("$($Global:__OrcaOsc133State.Esc)]133;C$($Global:__OrcaOsc133State.Bel)")
        return $commandLine
    }
}
`

export function getPowerShellOsc133Bootstrap(): string {
  return POWERSHELL_OSC133_BOOTSTRAP
}

export function isPowerShellExecutableName(shellName: string): boolean {
  const normalized = shellName.toLowerCase()
  return (
    normalized === 'pwsh' ||
    normalized === 'pwsh.exe' ||
    normalized === 'powershell' ||
    normalized === 'powershell.exe'
  )
}
```

## Verify

```bash
cd /opt/repos/orca/agent
npx tsc --noEmit -p tsconfig.json
```

Then add a minimal smoke test, `agent/src/relay/pty-osc133-bootstrap.test.ts`:

```typescript
import { describe, expect, it } from 'vitest'
import { getPowerShellOsc133Bootstrap, isPowerShellExecutableName, encodePowerShellCommand } from './pty-osc133-bootstrap'

describe('getPowerShellOsc133Bootstrap', () => {
  it('embeds OSC 133 A/B/C/D markers', () => {
    const script = getPowerShellOsc133Bootstrap()
    expect(script).toContain(']133;A')
    expect(script).toContain(']133;B')
    expect(script).toContain(']133;C')
    expect(script).toContain(']133;D')
  })

  it('base64/utf16le-encodes for -EncodedCommand', () => {
    const encoded = encodePowerShellCommand(getPowerShellOsc133Bootstrap())
    const decoded = Buffer.from(encoded, 'base64').toString('utf16le')
    expect(decoded).toContain('Orca OSC 133 shell integration for PowerShell')
  })
})

describe('isPowerShellExecutableName', () => {
  it('matches all four PowerShell executable name spellings', () => {
    expect(isPowerShellExecutableName('pwsh')).toBe(true)
    expect(isPowerShellExecutableName('PWSH.EXE')).toBe(true)
    expect(isPowerShellExecutableName('powershell')).toBe(true)
    expect(isPowerShellExecutableName('powershell.exe')).toBe(true)
    expect(isPowerShellExecutableName('bash')).toBe(false)
  })
})
```

```bash
npx vitest run src/relay/pty-osc133-bootstrap.test.ts
```

Expected: clean typecheck, all assertions pass.
