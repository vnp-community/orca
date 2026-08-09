/**
 * Regression guard for CR-TRACE-003 BL-TM-04 (TASK-BE-003.4).
 *
 * OSC 133 command-boundary scanning runs on every PTY output chunk — it is
 * explicitly OUT of scope for terminal tracing (CR-TRACE-000 §5 anti-over-
 * instrumentation: a per-chunk hot path must never carry span overhead).
 * This is a grep-based guard, not a feature test, so it fails loudly if a
 * future change accidentally wires tracing into the scan path.
 *
 * @module shared/__tests__/terminal-osc133-command-finished.test
 */

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

describe('OSC 133 scanning path — no tracing instrumentation (CR-TRACE-003 BL-TM-04)', () => {
  it('does not call Tracers.* or createTracer() anywhere in terminal-osc133-command-finished.ts', () => {
    const source = readFileSync(
      join(__dirname, '..', 'terminal-osc133-command-finished.ts'),
      'utf-8'
    )

    expect(source).not.toMatch(/Tracers\./)
    expect(source).not.toMatch(/createTracer\s*\(/)
    expect(source).not.toMatch(/from\s+['"].*shared\/trace/)
  })
})
