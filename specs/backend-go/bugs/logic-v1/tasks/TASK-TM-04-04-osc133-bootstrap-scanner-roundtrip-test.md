# TASK-TM-04-04: Round-trip test — bootstrap output against the existing OSC 133 scanner

**From Solution:** SOL-TM-04
**Priority:** P2 — regression guard, not required for the feature to function
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/pty-osc133-bootstrap-scanner.test.ts`
**Depends on:** TASK-TM-04-01 (bootstrap export)
**Status:** `[x]` DONE — pty-osc133-bootstrap-scanner.test.ts added; `npx vitest run src/relay/pty-osc133-bootstrap-scanner.test.ts` — 2/2 pass.

---

## Context

`agent/src/shared/terminal-osc133-command-finished.ts` is an existing,
already-tested, currently-unwired-under-`agent/src/relay/*.ts` chunk-boundary-safe
scanner for POSIX OSC 133;C/D sequences (`createOsc133CommandFinishedScanner`).
The ported PowerShell bootstrap (TASK-TM-04-01) emits the same OSC 133;A/B/C/D
wire format. This test proves the two independently-ported pieces agree on
that wire format — a regression guard against silent drift between them,
not a functional dependency (the scanner is not wired into the PowerShell
path by this solution; see SOL-TM-04's "explicitly out of scope" section).

## Changes to make

Create `agent/src/relay/pty-osc133-bootstrap-scanner.test.ts`:

```typescript
import { describe, expect, it } from 'vitest'
import { createOsc133CommandFinishedScanner } from '../shared/terminal-osc133-command-finished'

describe('PowerShell OSC 133 bootstrap output vs the shared scanner', () => {
  it('the scanner recognizes the bootstrap prompt function\'s emitted C/D sequences', () => {
    const started: number[] = []
    const finished: (number | null)[] = []
    const scanner = createOsc133CommandFinishedScanner(
      (exitCode) => finished.push(exitCode),
      () => started.push(started.length)
    )

    // Simulates one PowerShell prompt cycle's emitted bytes: prior command's
    // D (exit code 0) is skipped on the very first prompt (HasSeenPrompt is
    // false), so start the trace mid-session — this is what the bootstrap's
    // `Global:prompt` function and PSConsoleHostReadLine wrapper actually
    // write to the console, per pty-osc133-bootstrap.ts.
    const A = '\x1b]133;A\x07'
    const C = '\x1b]133;C\x07'
    const D = (code: number) => `\x1b]133;D;${code}\x07`

    scanner.scan(A) // prompt starts
    scanner.scan(C) // PSConsoleHostReadLine wrapper: command exec'd
    expect(started).toHaveLength(1)

    scanner.scan(D(0)) // next prompt's Global:prompt emits D for the finished command
    expect(finished).toEqual([0])
  })

  it('handles a non-zero exit code split across two chunks', () => {
    const finished: (number | null)[] = []
    const scanner = createOsc133CommandFinishedScanner((exitCode) => finished.push(exitCode))

    scanner.scan('\x1b]133;D;1')
    scanner.scan('27\x07') // BEL terminator arrives in a later chunk

    expect(finished).toEqual([127])
  })
})
```

## Verify

```bash
cd /opt/repos/orca/agent
npx tsc --noEmit -p tsconfig.json
npx vitest run src/relay/pty-osc133-bootstrap-scanner.test.ts
```

Expected: clean typecheck, both assertions pass — confirms the bootstrap's
`\x1b]133;<seq>[;<code>]\x07` format and the scanner's parser agree.
