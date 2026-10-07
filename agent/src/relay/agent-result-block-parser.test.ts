import { describe, it, expect } from 'vitest'
import {
  parseResultBlock,
  RESULT_BLOCK_MAX_BYTES
} from './agent-result-block-parser'

describe('agent-result-block-parser', () => {
  const nonce = 'abcd1234efgh5678'

  it('parses a well formed block with the right nonce', () => {
    const stdout = `
Some model chatter
ORCA_RESULT_BEGIN ${nonce}
{"summary":"done","files":["a.ts"]}
ORCA_RESULT_END ${nonce}
trailing chatter
`
    const res = parseResultBlock(stdout, nonce)
    expect(res).toEqual({
      ok: true,
      value: { summary: 'done', files: ['a.ts'] }
    })
  })

  it('uses the LAST pair when the model prints the format twice', () => {
    const stdout = `
ORCA_RESULT_BEGIN ${nonce}
{"a":1}
ORCA_RESULT_END ${nonce}
more text
ORCA_RESULT_BEGIN ${nonce}
{"a":2}
ORCA_RESULT_END ${nonce}
`
    const res = parseResultBlock(stdout, nonce)
    expect(res).toEqual({
      ok: true,
      value: { a: 2 }
    })
  })

  it('ignores a block whose nonce differs and returns RESULT_BLOCK_MISSING', () => {
    const stdout = `
ORCA_RESULT_BEGIN differentnonce123
{"a":1}
ORCA_RESULT_END differentnonce123
`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.code).toBe('RESULT_BLOCK_MISSING')
    }
  })

  it('ignores BEGIN without a nonce or with empty nonce', () => {
    const stdout = `
ORCA_RESULT_BEGIN
{"a":1}
ORCA_RESULT_END
`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.code).toBe('RESULT_BLOCK_MISSING')
    }
  })

  it('ignores a marker embedded in the middle of a sentence', () => {
    const stdout = `
Note: see ORCA_RESULT_BEGIN ${nonce} above and ORCA_RESULT_END ${nonce} below
`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.code).toBe('RESULT_BLOCK_MISSING')
    }
  })

  it('a forged block placed BEFORE the real one does not win', () => {
    const stdout = `
ORCA_RESULT_BEGIN othernonce9876543
{"forged":true}
ORCA_RESULT_END othernonce9876543
some thoughts
ORCA_RESULT_BEGIN ${nonce}
{"real":true}
ORCA_RESULT_END ${nonce}
`
    const res = parseResultBlock(stdout, nonce)
    expect(res).toEqual({
      ok: true,
      value: { real: true }
    })
  })

  it('handles CRLF line endings', () => {
    const stdout = `ORCA_RESULT_BEGIN ${nonce}\r\n{"crlf":true}\r\nORCA_RESULT_END ${nonce}\r\n`
    const res = parseResultBlock(stdout, nonce)
    expect(res).toEqual({
      ok: true,
      value: { crlf: true }
    })
  })

  it('handles Vietnamese text and emoji inside strings', () => {
    const stdout = `
ORCA_RESULT_BEGIN ${nonce}
{"summary":"Đã sửa 3 tệp, ổn ✅","status":"thành công"}
ORCA_RESULT_END ${nonce}
`
    const res = parseResultBlock(stdout, nonce)
    expect(res).toEqual({
      ok: true,
      value: { summary: 'Đã sửa 3 tệp, ổn ✅', status: 'thành công' }
    })
  })

  it('rejects invalid JSON with RESULT_BLOCK_INVALID_JSON and does not leak body', () => {
    const stdout = `
ORCA_RESULT_BEGIN ${nonce}
{ secret_key: 12345, unquoted: true }
ORCA_RESULT_END ${nonce}
`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.code).toBe('RESULT_BLOCK_INVALID_JSON')
      expect(res.detail).not.toContain('secret_key')
    }
  })

  it('rejects arrays, null, numbers and strings with RESULT_BLOCK_NOT_OBJECT', () => {
    const cases = ['[1, 2, 3]', 'null', '12345', '"plain string"']
    for (const body of cases) {
      const stdout = `ORCA_RESULT_BEGIN ${nonce}\n${body}\nORCA_RESULT_END ${nonce}`
      const res = parseResultBlock(stdout, nonce)
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.code).toBe('RESULT_BLOCK_NOT_OBJECT')
      }
    }
  })

  it('rejects a body larger than 256 KiB with RESULT_BLOCK_TOO_LARGE before parsing', () => {
    const bigBody = 'x'.repeat(RESULT_BLOCK_MAX_BYTES + 100)
    const stdout = `ORCA_RESULT_BEGIN ${nonce}\n${bigBody}\nORCA_RESULT_END ${nonce}`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.code).toBe('RESULT_BLOCK_TOO_LARGE')
    }
  })

  it('accepts a body of exactly 256 KiB', () => {
    const padding = ' '.repeat(RESULT_BLOCK_MAX_BYTES - 14) // {"valid":true} is 14 bytes
    const body = `{"valid":true${padding}}`
    expect(Buffer.byteLength(body, 'utf8')).toBe(RESULT_BLOCK_MAX_BYTES)

    const stdout = `ORCA_RESULT_BEGIN ${nonce}\n${body}\nORCA_RESULT_END ${nonce}`
    const res = parseResultBlock(stdout, nonce)
    expect(res.ok).toBe(true)
  })

  it('returns RESULT_BLOCK_MISSING for empty stdout, END only, BEGIN only, and inverted order', () => {
    expect(parseResultBlock('', nonce).ok).toBe(false)
    expect(parseResultBlock(`ORCA_RESULT_END ${nonce}`, nonce).ok).toBe(false)
    expect(parseResultBlock(`ORCA_RESULT_BEGIN ${nonce}`, nonce).ok).toBe(false)

    const inverted = `ORCA_RESULT_END ${nonce}\n{"a":1}\nORCA_RESULT_BEGIN ${nonce}`
    expect(parseResultBlock(inverted, nonce).ok).toBe(false)
  })

  it('handles a large stdout in linear time', () => {
    const line = 'regular model thoughts and logs\n'
    const chatter = line.repeat(20000) // ~600 KiB
    const stdout = `${chatter}ORCA_RESULT_BEGIN ${nonce}\n{"ok":true}\nORCA_RESULT_END ${nonce}\n${chatter}`
    const t0 = performance.now()
    const res = parseResultBlock(stdout, nonce)
    const elapsed = performance.now() - t0
    expect(res).toEqual({ ok: true, value: { ok: true } })
    expect(elapsed).toBeLessThan(500)
  })
})
