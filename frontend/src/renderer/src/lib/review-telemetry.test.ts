import { beforeEach, describe, expect, it, vi } from 'vitest'

const track = vi.hoisted(() => vi.fn())
vi.mock('./telemetry', () => ({ track }))

import { eventSchemas } from '../../../shared/telemetry-events'
import {
  bucketCount,
  bucketDwellMs,
  bucketLarge,
  bucketLatencyMs,
  toToolBucket,
  trackQualityFindingTriaged,
  trackQualityGateViewed,
  trackReviewAiSummary,
  trackReviewDecisionMade,
  trackReviewFindingsSummary,
  trackReviewLensViewed,
  trackReviewOpened,
  trackReviewReportExported
} from './review-telemetry'

beforeEach(() => track.mockClear())

describe('buckets', () => {
  it.each([
    [0, '0'], [1, '1'], [2, '2-3'], [3, '2-3'], [4, '4-10'], [10, '4-10'], [11, '11+'], [999, '11+'], [-5, '0'], [Number.NaN, '0'], [2.9, '2-3']
  ])('bucketCount(%s) = %s', (n, expected) => expect(bucketCount(n)).toBe(expected))

  it.each([
    [0, '0'], [1, '1-3'], [3, '1-3'], [4, '4-10'], [10, '4-10'], [11, '11-30'], [30, '11-30'], [31, '31+']
  ])('bucketLarge(%s) = %s', (n, expected) => expect(bucketLarge(n)).toBe(expected))

  it.each([
    [0, '<1m'], [59_999, '<1m'], [60_000, '<5m'], [299_999, '<5m'], [300_000, '<30m'], [1_800_000, '<4h'], [14_400_000, '>=4h']
  ])('bucketLatencyMs(%s) = %s', (ms, expected) => expect(bucketLatencyMs(ms)).toBe(expected))

  it.each([
    [0, '<10s'], [9_999, '<10s'], [10_000, '<1m'], [60_000, '<5m'], [300_000, '>=5m']
  ])('bucketDwellMs(%s) = %s', (ms, expected) => expect(bucketDwellMs(ms)).toBe(expected))

  it('toToolBucket keeps known tools and maps everything else to other', () => {
    expect(toToolBucket('ESLint')).toBe('eslint')
    expect(toToolBucket('go test')).toBe('go_test')
    expect(toToolBucket('golangci-lint')).toBe('golangci_lint')
    expect(toToolBucket('my-secret-internal-tool')).toBe('other')
    expect(toToolBucket('')).toBe('other')
    expect(toToolBucket(null)).toBe('other')
  })
})

describe('wrappers emit schema-valid payloads', () => {
  function lastCall(): [string, unknown] {
    return track.mock.calls.at(-1) as [string, unknown]
  }
  function expectValid(): void {
    const [name, props] = lastCall()
    const result = eventSchemas[name as keyof typeof eventSchemas].safeParse(props)
    expect(result.success, JSON.stringify(result.success ? '' : result.error.issues)).toBe(true)
  }

  it('covers all eight events', () => {
    trackReviewOpened({ source: 'cmd_k', scope: 'merge_base', lens: 'impact', index: 'fresh', after_agent_turn: true })
    expectValid()
    trackReviewLensViewed({ lens: 'quality', dwellMs: 20_000 })
    expectValid()
    trackQualityGateViewed({ verdict: 'fail', reasonCount: 7, stale: false, surface: 'source_control', source: 'local' })
    expectValid()
    trackReviewDecisionMade({ decision: 'abandon', latencyMs: 5_000_000, usedReview: false, gate: 'none', openFindings: 12 })
    expectValid()
    trackReviewFindingsSummary({ shown: 40, dismissed: 3, waived: 1, resolved: 0 })
    expectValid()
    trackQualityFindingTriaged({ action: 'waive', reason: 'later', severity: 'error', tool: 'weird-tool', blocking: true })
    expectValid()
    trackReviewReportExported({ format: 'html_save', provider: 'gitlab', truncated: false })
    expectValid()
    trackReviewAiSummary({ outcome: 'ok', level: 'metadata', cache_hit: false, feedback: 'none' })
    expectValid()
    expect(track).toHaveBeenCalledTimes(8)
  })

  it('never lets a raw count or free-form tool name through', () => {
    trackQualityGateViewed({ verdict: 'pass', reasonCount: 123456, stale: true, surface: 'review', source: 'ci' })
    expect(lastCall()[1]).toMatchObject({ reasons: '11+' })
    trackQualityFindingTriaged({ action: 'dismiss', reason: 'other', severity: 'info', tool: '/home/user/secret-tool', blocking: false })
    expect(lastCall()[1]).toMatchObject({ tool: 'other' })
  })
})
