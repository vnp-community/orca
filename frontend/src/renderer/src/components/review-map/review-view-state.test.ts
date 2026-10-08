import { describe, expect, it } from 'vitest'
import { computeReviewViewState, type ReviewViewStateInput } from './review-view-state'
import { makeOverlay, makeStatus } from './review-test-data'
import type { ReviewError } from './review-wire-types'

const err = (kind: ReviewError['kind']): ReviewError => ({ kind, message: kind })

function input(over: Partial<ReviewViewStateInput> = {}): ReviewViewStateInput {
  return {
    support: 'enabled',
    selectorUnsupported: false,
    scope: {
      status: 'ready',
      scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true }
    },
    status: { data: makeStatus(), error: null },
    overlay: { data: makeOverlay(), error: null, pending: false },
    hasNewDataSignal: false,
    ...over
  }
}
const screenId = (i: ReviewViewStateInput): string | null => {
  const v = computeReviewViewState(i)
  return v.kind === 'screen' ? v.screen.id : null
}

describe('computeReviewViewState', () => {
  it('ready with no banners', () => {
    expect(computeReviewViewState(input())).toEqual({ kind: 'ready', banners: [] })
  })
  it('1 unsupported/disabled win over everything', () => {
    expect(
      screenId(
        input({
          support: 'disabled',
          overlay: { data: null, error: err('no-binding'), pending: false }
        })
      )
    ).toBe('disabled')
    expect(screenId(input({ support: 'unsupported' }))).toBe('unsupported')
    expect(screenId(input({ selectorUnsupported: true }))).toBe('unsupported')
  })
  it('2 scope-error beats no-binding', () => {
    expect(
      screenId(
        input({
          scope: { status: 'blocked', reason: 'invalid-base' },
          overlay: { data: null, error: err('no-binding'), pending: false }
        })
      )
    ).toBe('scope-error')
  })
  it('3 no-binding beats index-missing', () => {
    expect(
      screenId(
        input({
          overlay: { data: null, error: err('no-binding'), pending: false },
          status: { data: makeStatus({ overall: 'MISSING' }), error: null }
        })
      )
    ).toBe('no-binding')
  })
  it('4 forbidden', () => {
    expect(
      screenId(input({ overlay: { data: null, error: err('forbidden'), pending: false } }))
    ).toBe('forbidden')
  })
  it('5 loading-initial while nothing arrived', () => {
    expect(
      screenId(
        input({
          overlay: { data: null, error: null, pending: true },
          status: { data: null, error: null }
        })
      )
    ).toBe('loading-initial')
    expect(
      screenId(
        input({
          scope: { status: 'loading' },
          overlay: { data: null, error: null, pending: false }
        })
      )
    ).toBe('loading-initial')
  })
  it('6 tool-unavailable from error or NOT_INSTALLED', () => {
    expect(
      screenId(input({ overlay: { data: null, error: err('tool-unavailable'), pending: false } }))
    ).toBe('tool-unavailable')
    expect(
      screenId(
        input({
          status: { data: makeStatus({ overall: 'NOT_INSTALLED' }), error: null },
          overlay: { data: null, error: null, pending: false }
        })
      )
    ).toBe('tool-unavailable')
  })
  it('7 repo-not-registered and path-not-allowed', () => {
    expect(
      screenId(
        input({ overlay: { data: null, error: err('repo-not-registered'), pending: false } })
      )
    ).toBe('repo-not-registered')
    expect(
      screenId(input({ overlay: { data: null, error: err('path-not-allowed'), pending: false } }))
    ).toBe('path-not-allowed')
  })
  it('8 index-missing from error or MISSING', () => {
    expect(
      screenId(input({ overlay: { data: null, error: err('index-missing'), pending: false } }))
    ).toBe('index-missing')
    expect(
      screenId(
        input({
          status: { data: makeStatus({ overall: 'MISSING' }), error: null },
          overlay: { data: null, error: null, pending: false }
        })
      )
    ).toBe('index-missing')
  })
  it('9 index-building carries percent (null stays null)', () => {
    const v = computeReviewViewState(
      input({
        status: {
          data: makeStatus({
            overall: 'BUILDING',
            activeJob: { id: 'j', stage: 's', percent: null }
          }),
          error: null
        },
        overlay: { data: null, error: null, pending: false }
      })
    )
    expect(v).toEqual({
      kind: 'screen',
      screen: { id: 'index-building', percent: null, stage: 's' }
    })
  })
  it('10 offline: screen without cache, banner with cache', () => {
    expect(
      screenId(input({ overlay: { data: null, error: err('offline'), pending: false } }))
    ).toBe('offline')
    const v = computeReviewViewState(
      input({ overlay: { data: makeOverlay(), error: err('offline'), pending: false } })
    )
    expect(v).toEqual({ kind: 'ready', banners: [{ id: 'offline-cached' }] })
  })
  it('11 error screen without data, banner with data', () => {
    expect(
      screenId(input({ overlay: { data: null, error: err('tool-failed'), pending: false } }))
    ).toBe('error')
    expect(
      screenId(input({ overlay: { data: null, error: err('timeout'), pending: false } }))
    ).toBe('error')
    const v = computeReviewViewState(
      input({ overlay: { data: makeOverlay(), error: err('tool-failed'), pending: false } })
    )
    expect(v.kind === 'ready' && v.banners[0].id).toBe('error-cached')
  })
  it('12 no-changes, including unborn-head', () => {
    expect(
      computeReviewViewState(
        input({ overlay: { data: makeOverlay({ changedFiles: [] }), error: null, pending: false } })
      )
    ).toEqual({ kind: 'screen', screen: { id: 'no-changes', reason: null } })
    expect(
      computeReviewViewState(
        input({
          overlay: {
            data: makeOverlay({ changedFiles: [], emptyReason: 'unborn-head' }),
            error: null,
            pending: false
          }
        })
      )
    ).toEqual({ kind: 'screen', screen: { id: 'no-changes', reason: 'unborn-head' } })
  })
  it('13 banners: stale, truncated, scope-mismatch, new-data', () => {
    const v = computeReviewViewState(
      input({
        status: { data: makeStatus({ overall: 'STALE', scopeMismatch: true }), error: null },
        overlay: {
          data: makeOverlay({
            limits: { truncated: { files: true }, totalCounts: { changedFiles: 900 } }
          }),
          error: null,
          pending: false
        },
        hasNewDataSignal: true
      })
    )
    expect(v.kind === 'ready' && v.banners.map((b) => b.id)).toEqual([
      'stale',
      'truncated',
      'scope-mismatch',
      'new-data'
    ])
    expect(v.kind === 'ready' && v.banners[1]).toEqual({ id: 'truncated', shown: 3, total: 900 })
  })
})
