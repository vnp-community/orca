import { describe, expect, it } from 'vitest'
import { readMobileReviewSummaryResult } from './mobile-review-summary-rpc'

const finding = {
  key: 'f1',
  kind: 'layering',
  severity: 'error',
  title: 'usecase to adapter',
  summary: 'bad import',
  origin: 'introduced',
  filePath: 'a/b.go',
  startLine: 88,
  inChangedFiles: true
}

describe('readMobileReviewSummaryResult', () => {
  it('parses a full payload and ignores unknown fields', () => {
    const parsed = readMobileReviewSummaryResult({
      available: true,
      extra: 1,
      index: {
        state: 'ready',
        indexedCommit: 'abc',
        headCommit: 'def',
        indexedAt: 't'
      },
      counts: {
        files: 12,
        symbols: 38,
        flows: 3,
        tables: 2,
        contracts: 1,
        uncovered: 5
      },
      risk: { level: 'MEDIUM', reasons: ['touches 3 flows'] },
      findings: {
        totalOpen: 5,
        bySeverity: { error: 2, warning: 1, info: 2, high: 9 },
        items: [finding],
        truncated: false
      },
      stale: true,
      headCommit: 'def'
    })
    expect(parsed?.available).toBe(true)
    expect(parsed?.counts?.symbols).toBe(38)
    expect(parsed?.risk).toEqual({
      level: 'MEDIUM',
      reasons: ['touches 3 flows']
    })
    expect(parsed?.findings?.bySeverity).toEqual({
      error: 2,
      warning: 1,
      info: 2
    })
    expect(parsed?.findings?.items[0]?.startLine).toBe(88)
    expect(parsed?.stale).toBe(true)
  })

  it('tolerates missing optional sections', () => {
    const parsed = readMobileReviewSummaryResult({ available: true })
    expect(parsed).toMatchObject({ available: true })
    expect(parsed?.counts).toBeUndefined()
    expect(parsed?.findings).toBeUndefined()
  })

  it.each(['flag_off', 'no_binding', 'index_missing', 'tool_unavailable'])(
    'keeps reason %s',
    (reason) => {
      expect(readMobileReviewSummaryResult({ available: false, reason })?.reason).toBe(reason)
    }
  )

  it('drops an unknown reason', () => {
    expect(readMobileReviewSummaryResult({ available: false, reason: 'weird' })?.reason).toBe(
      undefined
    )
  })

  it('coerces unknown enums', () => {
    const parsed = readMobileReviewSummaryResult({
      available: true,
      risk: { level: 'SEVERE', reasons: [1, 'ok'] },
      findings: {
        items: [{ ...finding, severity: 'high', origin: 'elsewhere' }]
      }
    })
    expect(parsed?.risk).toEqual({ level: 'UNKNOWN', reasons: ['ok'] })
    expect(parsed?.findings?.items[0]).toMatchObject({
      severity: 'info',
      origin: 'unknown'
    })
  })

  it('truncates long strings and long arrays, clamps negatives', () => {
    const items = Array.from({ length: 60 }, (_, i) => ({
      ...finding,
      key: `k${i}`,
      title: 'x'.repeat(500),
      summary: 'y'.repeat(900)
    }))
    const parsed = readMobileReviewSummaryResult({
      available: true,
      counts: { files: -3 },
      findings: { totalOpen: 60, items }
    })
    expect(parsed?.counts?.files).toBe(0)
    expect(parsed?.findings?.items).toHaveLength(50)
    expect(parsed?.findings?.items[0]?.title).toHaveLength(200)
    expect(parsed?.findings?.items[0]?.summary).toHaveLength(400)
    expect(parsed?.findings?.truncated).toBe(true)
  })

  it('skips findings without a key and never throws on bad input', () => {
    const parsed = readMobileReviewSummaryResult({
      available: true,
      findings: { items: [null, 3, { title: 'no key' }, finding] }
    })
    expect(parsed?.findings?.items).toHaveLength(1)
    for (const bad of [null, undefined, 'x', 4, [], {}, { available: 'yes' }]) {
      expect(readMobileReviewSummaryResult(bad)).toBeNull()
    }
  })
})
