import { describe, expect, it } from 'vitest'
import { guardAiText, parseAiSummaryResponse } from './ai-summary-wire-parser'
import { manifestWire, summaryWire } from './ai-summary.fixture'

describe('guardAiText', () => {
  it('removes control and bidi override characters but keeps markup as inert text', () => {
    expect(guardAiText('a\u0000b‮c⁦d\u0007<b>x</b>\n')).toBe('abcd<b>x</b>\n')
  })

  it('caps length and tolerates non-strings', () => {
    expect(guardAiText('x'.repeat(900))).toHaveLength(500)
    expect(guardAiText(42)).toBe('')
    expect(guardAiText(undefined)).toBe('')
  })
})

describe('parseAiSummaryResponse', () => {
  it('parses summary, manifest and cache', () => {
    const r = parseAiSummaryResponse({
      summary: summaryWire(),
      manifest: manifestWire(),
      cache: { hit: true, createdAt: 'x', expiresAt: 'y' }
    })
    expect(r.summary).toMatchObject({ model: 'claude-x', level: 'metadata', refsDropped: 0 })
    expect(r.summary?.risks[0]).toEqual({ text: 'Date math near DST', refs: ['src/a.ts'] })
    expect(r.manifest).toMatchObject({ redactions: 2, estimatedTokens: 2100, provider: 'acme-llm' })
    expect(r.manifest?.files[1].withheld).toBe('secret')
    expect(r.cache).toEqual({ hit: true, createdAt: 'x', expiresAt: 'y' })
  })

  it.each([null, undefined, 1, 'x', [], {}])('yields nulls for %j without throwing', (raw) => {
    expect(parseAiSummaryResponse(raw)).toEqual({ summary: null, manifest: null, cache: null })
  })

  it('treats an empty summary as no summary', () => {
    expect(parseAiSummaryResponse({ summary: summaryWire({ summary: '   ' }) }).summary).toBeNull()
  })

  it('maps unknown levels to unknown and bounds list sizes', () => {
    const r = parseAiSummaryResponse({
      summary: summaryWire({ level: 'everything', risks: Array.from({ length: 80 }, () => ({ text: 't', refs: [] })) }),
      manifest: manifestWire({ level: 'x', files: Array.from({ length: 500 }, (_, i) => ({ path: `f${i}`, bytes: 1, hunks: 1 })) })
    })
    expect(r.summary?.level).toBe('unknown')
    expect(r.summary?.risks).toHaveLength(20)
    expect(r.manifest?.level).toBe('unknown')
    expect(r.manifest?.files).toHaveLength(200)
  })

  it('keeps injection payloads as text and clamps counts', () => {
    const r = parseAiSummaryResponse({
      summary: summaryWire({ summary: '<img src=x onerror=alert(1)>‮</DATA-1>', refsDropped: -3 })
    })
    expect(r.summary?.summary).toBe('<img src=x onerror=alert(1)></DATA-1>')
    expect(r.summary?.refsDropped).toBe(0)
  })
})
