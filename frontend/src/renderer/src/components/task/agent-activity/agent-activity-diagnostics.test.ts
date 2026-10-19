import { describe, expect, it } from 'vitest'
import { parseAgentOutput } from './agent-activity-lines'

// BUG-QA-05 (specs/frontend/bugs/qa-2026-10-16): the Claude Code CLI prints `[claude-code:unrecognized_model] {...}` on stderr when
// it does not know a model id, although the run itself finished. The panel draws every stderr line as an error.
// Fixed by SOL-QA-05: such a line is a diagnostic, an error only when the run failed or the code is a blocker.
const DONE = '■ Done · 6 turns · 13.8s'
const DIAGNOSTIC =
  '[claude-code:unrecognized_model] {"model":"claude-sonnet-5-5","query_source":"sdk"}'

describe('CLI diagnostics in the agent log (BUG-QA-05)', () => {
  it('after a run that finished, a [claude-code:…] diagnostic is not an error line', () => {
    const out = parseAgentOutput(
      `· Session started (claude-sonnet-5-5)\n${DONE}\n`,
      `${DIAGNOSTIC}\n`
    )
    const diag = out.lines.find((l) => l.text.includes('unrecognized_model'))
    expect(diag, 'the diagnostic is still shown').toBeDefined()
    expect(diag?.kind).toBe('diagnostic')
    expect(diag?.tone).not.toBe('error')
  })

  it('a [claude-code:…] diagnostic of a run that failed stays an error', () => {
    const out = parseAgentOutput('· Session started (m)\n■ Failed · 2 turns\n', `${DIAGNOSTIC}\n`)
    const diag = out.lines.find((l) => l.text.includes('unrecognized_model'))
    expect(diag?.kind).toBe('diagnostic')
    expect(diag?.tone).toBe('error')
  })

  it('a blocking code is an error even when the run finished', () => {
    const out = parseAgentOutput(`${DONE}\n`, '[claude-code:rate_limited] {"retryAfter":30}\n')
    expect(out.lines.at(-1)).toMatchObject({ kind: 'diagnostic', tone: 'error' })
  })

  it('stderr without a tag keeps being an error line (unchanged by the fix)', () => {
    const out = parseAgentOutput(`${DONE}\n`, 'Error: ENOENT: no such file\n')
    const err = out.lines.find((l) => l.text.startsWith('Error: ENOENT'))
    expect(err?.kind).toBe('stderr')
  })
})
