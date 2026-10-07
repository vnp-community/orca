/**
 * review-telemetry.test.ts — FE-CV-TASK-095-03
 *
 * Tests for review telemetry bucket helpers.
 * Mocks track() to verify payloads stay coarse.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  bucketCount,
  bucketLarge,
  bucketLatencyMs,
  bucketDwellMs,
  toToolBucket,
  trackReviewTabOpened,
  trackReviewLensChanged,
  trackReviewReportExported,
} from './review-telemetry'

vi.mock('./telemetry', () => ({ track: vi.fn() }))

import { track } from './telemetry'
const mockTrack = track as ReturnType<typeof vi.fn>

beforeEach(() => {
  mockTrack.mockClear()
})

// ---------------------------------------------------------------------------
// Bucket helpers
// ---------------------------------------------------------------------------

describe('bucketCount', () => {
  it('returns 0 for n=0', () => expect(bucketCount(0)).toBe('0'))
  it('returns 1-5 for n=1', () => expect(bucketCount(1)).toBe('1-5'))
  it('returns 1-5 for n=5', () => expect(bucketCount(5)).toBe('1-5'))
  it('returns 6-20 for n=6', () => expect(bucketCount(6)).toBe('6-20'))
  it('returns 6-20 for n=20', () => expect(bucketCount(20)).toBe('6-20'))
  it('returns 21-50 for n=21', () => expect(bucketCount(21)).toBe('21-50'))
  it('returns 50+ for n=51', () => expect(bucketCount(51)).toBe('50+'))
})

describe('bucketLarge', () => {
  it('0→0', () => expect(bucketLarge(0)).toBe('0'))
  it('1→1', () => expect(bucketLarge(1)).toBe('1'))
  it('2→2-5', () => expect(bucketLarge(2)).toBe('2-5'))
  it('5→2-5', () => expect(bucketLarge(5)).toBe('2-5'))
  it('6→6-20', () => expect(bucketLarge(6)).toBe('6-20'))
  it('20→6-20', () => expect(bucketLarge(20)).toBe('6-20'))
  it('21→20+', () => expect(bucketLarge(21)).toBe('20+'))
})

describe('bucketLatencyMs', () => {
  it('<1s', () => expect(bucketLatencyMs(999)).toBe('<1s'))
  it('<5s', () => expect(bucketLatencyMs(1000)).toBe('<5s'))
  it('<15s', () => expect(bucketLatencyMs(5000)).toBe('<15s'))
  it('<60s', () => expect(bucketLatencyMs(15000)).toBe('<60s'))
  it('>=60s', () => expect(bucketLatencyMs(60000)).toBe('>=60s'))
})

describe('bucketDwellMs', () => {
  it('<5s for ms=4999', () => expect(bucketDwellMs(4999)).toBe('<5s'))
  it('<30s for ms=5000', () => expect(bucketDwellMs(5000)).toBe('<30s'))
  it('<2m for ms=30000', () => expect(bucketDwellMs(30000)).toBe('<2m'))
  it('>=2m for ms=120000', () => expect(bucketDwellMs(120000)).toBe('>=2m'))
})

describe('toToolBucket', () => {
  it('known tool → correct bucket', () => expect(toToolBucket('commit')).toBe('commit'))
  it('unknown tool → other', () => expect(toToolBucket('something_random')).toBe('other'))
  it('empty string → other', () => expect(toToolBucket('')).toBe('other'))
})

// ---------------------------------------------------------------------------
// Track wrappers
// ---------------------------------------------------------------------------

describe('trackReviewTabOpened', () => {
  it('calls track with coarse payload', () => {
    trackReviewTabOpened({ worktreeFingerprint: 'fp1', trigger: 'click' })
    expect(mockTrack).toHaveBeenCalledOnce()
    const [event, payload] = mockTrack.mock.calls[0]
    expect(event).toBe('review_tab_opened')
    expect(payload.worktree_fingerprint).toBe('fp1')
    expect(payload.trigger).toBe('click')
    // No raw paths, no model/agent_type
    expect(payload).not.toHaveProperty('model')
    expect(payload).not.toHaveProperty('agent_type')
  })
})

describe('trackReviewLensChanged', () => {
  it('includes lens_id and previous_lens_id', () => {
    trackReviewLensChanged({
      worktreeFingerprint: 'fp2',
      lensId: 'impact',
      previousLensId: 'structure',
    })
    expect(mockTrack).toHaveBeenCalledOnce()
    const [, payload] = mockTrack.mock.calls[0]
    expect(payload.lens_id).toBe('impact')
    expect(payload.previous_lens_id).toBe('structure')
  })
})

describe('trackReviewReportExported', () => {
  it('buckets section_count', () => {
    trackReviewReportExported({
      worktreeFingerprint: 'fp3',
      format: 'markdown',
      sectionCount: 7,
    })
    const [, payload] = mockTrack.mock.calls[0]
    expect(payload.format).toBe('markdown')
    expect(payload.section_count).toBe('6-20')
  })
})
