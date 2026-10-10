import { describe, expect, it } from 'vitest'
import { MAX_ACTIVITY_LINES, maskSecretShapes, parseAgentOutput } from './agent-activity-lines'

describe('parseAgentOutput', () => {
  it('classifies lines by their leading glyph and drops the glyph from the text', () => {
    const out = parseAgentOutput(
      [
        '· Session started (m)',
        '» Reading the file',
        '▸ Read src/calc.py',
        '↳ ok · 3 lines',
        '■ Done · 2 turns · 5.7s',
        ''
      ].join('\n'),
      ''
    )
    expect(out.lines.map((l) => [l.kind, l.text])).toEqual([
      ['info', 'Session started (m)'],
      ['text', 'Reading the file'],
      ['tool', 'Read src/calc.py'],
      ['result', 'ok · 3 lines'],
      ['end', 'Done · 2 turns · 5.7s']
    ])
    expect(out.olderHidden).toBe(false)
  })

  it('marks failures as errors and a clean finish as ok', () => {
    const out = parseAgentOutput(
      '↳ error: Exit code 1\n■ Failed: Not logged in\n■ Done\n↳ ok · 1 lines',
      ''
    )
    expect(out.lines.map((l) => l.tone)).toEqual(['error', 'error', 'ok', undefined])
  })

  it('shows text without a glyph as raw and ignores blank lines and CRLF', () => {
    const out = parseAgentOutput('plain line\r\n\r\n   \r\n» next', '')
    expect(out.lines.map((l) => [l.kind, l.text])).toEqual([
      ['raw', 'plain line'],
      ['text', 'next']
    ])
  })

  it('puts stderr lines after stdout, keeping only the last ones', () => {
    const err = Array.from({ length: 30 }, (_, i) => `warn ${i}`).join('\n')
    const out = parseAgentOutput('» hi', err)
    const stderr = out.lines.filter((l) => l.kind === 'stderr')
    expect(stderr).toHaveLength(20)
    expect(stderr[0]?.text).toBe('warn 10')
    expect(out.lines[0]?.kind).toBe('text')
  })

  it('keeps a bounded window of the newest lines with stable ids', () => {
    const total = MAX_ACTIVITY_LINES + 50
    const text = Array.from({ length: total }, (_, i) => `» line ${i}`).join('\n')
    const out = parseAgentOutput(text, '')
    expect(out.lines).toHaveLength(MAX_ACTIVITY_LINES)
    expect(out.olderHidden).toBe(true)
    expect(out.lines[0]).toMatchObject({ id: 50, text: 'line 50' })
    expect(out.lines.at(-1)).toMatchObject({ id: total - 1, text: `line ${total - 1}` })
  })

  it('bounds a huge output and does not show the half line the cut left', () => {
    const big = `${'» x'.repeat(200_000)}\n» last line`
    const out = parseAgentOutput(big, '')
    expect(out.olderHidden).toBe(true)
    expect(out.lines.map((l) => l.text)).toEqual(['last line'])
  })

  it('truncates a very long line', () => {
    const out = parseAgentOutput(`» ${'a'.repeat(2000)}`, '')
    expect(out.lines[0]?.text.length).toBeLessThanOrEqual(400)
    expect(out.lines[0]?.text.endsWith('…')).toBe(true)
  })

  it('masks credential shapes in stdout and stderr, and never renders markup as anything but text', () => {
    const out = parseAgentOutput(
      '» key sk-ant-abcdefghijklmnopqrstuvwxyz01\n▸ Bash: curl -H "Authorization: Bearer abcdefghijklmnop1234"\n» <script>alert(1)</script>',
      'oops ghp_abcdefghijklmnopqrstuvwxyz0123'
    )
    const all = out.lines.map((l) => l.text).join('\n')
    expect(all).not.toContain('sk-ant')
    expect(all).not.toContain('abcdefghijklmnop1234')
    expect(all).not.toContain('ghp_')
    expect(all).toContain('<script>alert(1)</script>')
  })
})

describe('maskSecretShapes', () => {
  it('masks a private key block, JWTs and AWS key ids', () => {
    expect(maskSecretShapes('-----BEGIN RSA PRIVATE KEY-----\nMIIE')).toBe('***')
    expect(maskSecretShapes('t eyJhbGciOi.eyJzdWIiOi.abc123_-x ok')).toBe('t *** ok')
    expect(maskSecretShapes('id AKIAABCDEFGHIJKLMNOP.')).toBe('id ***.')
  })
})
