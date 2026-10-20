import { describe, expect, it } from 'vitest'
import { makeItem, makeModel, makeReview, makeRun, makeVerdict } from './outcome-review.fixture'
import {
  bannersOf,
  designCardOf,
  drawnSectionsOf,
  failedContextRun,
  gateOf,
  needsDecisionItems,
  phaseOfRequest,
  releaseReadinessOf,
  requirementsCardOf,
  reviewHintOf,
  riskOf,
  screenOf,
  timelineRowStateOf,
  type ReviewViewInput
} from './outcome-review-view-model'
import { agentRunFactsOf } from './outcome-review-run-facts'
import type { RequestStatus } from '../../../../../shared/request-types'

function input(over: Partial<ReviewViewInput> = {}): ReviewViewInput {
  return {
    status: 'ready',
    review: makeReview(),
    estimate: null,
    requestStatus: 'completed',
    activeRun: null,
    runs: [],
    ...over
  }
}

describe('screenOf', () => {
  it('follows the hook status and the phase of the Request', () => {
    expect(screenOf(input({ status: 'loading', review: null }))).toBe('loading')
    for (const status of ['unsupported', 'disabled', 'not_ready'] as const) {
      expect(screenOf(input({ status, review: null }))).toBe('unavailable')
    }
    expect(screenOf(input({ status: 'error', review: null }))).toBe('error')
    expect(screenOf(input({ status: 'none', review: null, requestStatus: 'executing' }))).toBe(
      'invite_design'
    )
    expect(screenOf(input({ status: 'none', review: null, requestStatus: 'completed' }))).toBe(
      'invite_delivered'
    )
    expect(screenOf(input())).toBe('review')
    expect(screenOf(input({ review: null }))).toBe('loading')
  })

  it('names the phase a review made now would have', () => {
    const phases: [RequestStatus, string][] = [
      ['awaiting_plan_approval', 'design'],
      ['executing', 'design'],
      ['completed', 'delivered']
    ]
    for (const [status, phase] of phases) {
      expect(phaseOfRequest(status)).toBe(phase)
    }
  })
})

describe('bannersOf', () => {
  it('a design review before the code: only the design banner', () => {
    const review = makeReview({ phase: 'design' })
    expect(bannersOf(input({ review, requestStatus: 'executing' }))).toEqual(['design_phase'])
  })

  it('a design review after Execution asks for the after-code review', () => {
    const review = makeReview({ phase: 'design' })
    expect(bannersOf(input({ review, requestStatus: 'completed' }))).toEqual(['execution_finished'])
  })

  it('stale, truncated and failed drawings each have their banner, in a fixed order', () => {
    const review = makeReview({
      state: 'stale',
      model: {
        truncated: true,
        diagrams: [
          {
            section: 'flow',
            variant: 'delta',
            type: 'sequence',
            inputDigest: 'x',
            status: 'failed'
          }
        ]
      }
    })
    expect(bannersOf(input({ review }))).toEqual(['stale', 'drawing_failed', 'truncated'])
  })

  it('an unavailable graph says so and does not also call the design unreviewed', () => {
    const review = makeReview({ model: { kg: { available: false, gaps: [], conflicts: [] } } })
    expect(bannersOf(input({ review }))).toEqual(['kg_unavailable'])
  })

  it('a low share of approved design elements raises the advisory banner, a high one does not', () => {
    const low = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          architecture: makeVerdict({ designChecked: true, reviewedRatio: 0.25 })
        }
      }
    })
    expect(bannersOf(input({ review: low }))).toEqual(['design_not_reviewed'])
    const unchecked = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          architecture: makeVerdict({ designChecked: false, reviewedRatio: 0 })
        }
      }
    })
    expect(bannersOf(input({ review: unchecked }))).toEqual([])
    expect(bannersOf(input())).toEqual([])
  })

  it('a failed context run is said once nothing newer is going', () => {
    const runs = [
      makeRun({ status: 'failed', errorCode: 'REQUEST_REVIEW_RESULT_INVALID' }),
      makeRun({ runId: 'old', status: 'succeeded' })
    ]
    expect(failedContextRun(runs, null)?.errorCode).toBe('REQUEST_REVIEW_RESULT_INVALID')
    expect(failedContextRun(runs, makeRun({ status: 'running' }))).toBeNull()
    expect(failedContextRun([makeRun({ kind: 'draw', status: 'failed' })], null)).toBeNull()
    expect(bannersOf(input({ runs }))).toEqual(['run_failed'])
  })

  it('has no banner without a review', () => {
    expect(bannersOf(input({ review: null }))).toEqual([])
  })
})

describe('designCardOf', () => {
  it('after the code: counts of the code against the design', () => {
    const review = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          architecture: makeVerdict({
            designChecked: true,
            containersChanged: 2,
            newEdges: 3,
            removedEdges: 1,
            unplannedEdges: 1,
            reviewedRatio: 0.5
          })
        }
      }
    })
    const card = designCardOf(review, 'architecture')
    expect(card.mode).toBe('delivered')
    expect(card.agentReported).toBe(true)
    expect(card.checkedAgainstDesign).toBe(true)
    expect(card.snapshot).toBe('snap-000004')
    expect(card.rows).toEqual([
      { id: 'containers_changed', value: 2 },
      { id: 'new_edges', value: 3 },
      { id: 'removed_edges', value: 1 },
      { id: 'unplanned_edges', value: 1 }
    ])
  })

  it('without a design to compare with the "not in design" count is Unknown, never 0', () => {
    const review = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          flow: makeVerdict({ designChecked: false, newCalls: 2 })
        }
      }
    })
    const card = designCardOf(review, 'flow')
    expect(card.checkedAgainstDesign).toBe(false)
    expect(card.rows.find((r) => r.id === 'unplanned_calls')?.value).toBeNull()
    expect(card.rows.find((r) => r.id === 'new_calls')?.value).toBe(2)
  })

  it('before the code: the design itself, counted from what the review reports', () => {
    const review = makeReview({
      phase: 'design',
      model: {
        items: [
          makeItem({ section: 'architecture', key: 'a' }),
          makeItem({ section: 'architecture', key: 'b' }),
          makeItem({ section: 'flow', key: 'c' })
        ],
        diagrams: [
          {
            section: 'architecture',
            variant: 'design',
            type: 'architecture',
            inputDigest: 'd',
            status: 'cached'
          }
        ]
      }
    })
    expect(designCardOf(review, 'architecture')).toMatchObject({
      mode: 'design',
      checkedAgainstDesign: false,
      rows: [
        { id: 'design_items', value: 2 },
        { id: 'drawings', value: 1 }
      ]
    })
    expect(designCardOf(review, 'flow').rows).toEqual([
      { id: 'design_items', value: 1 },
      { id: 'drawings', value: 0 }
    ])
  })

  it('no snapshot is shown when the graph is not available', () => {
    const review = makeReview({
      model: { kg: { available: false, snapshot: 'stale-id', gaps: [], conflicts: [] } }
    })
    expect(designCardOf(review, 'architecture').snapshot).toBeUndefined()
  })
})

describe('verdict cards', () => {
  it('requirements: all delivered means every criterion has a passing check, nothing more', () => {
    const all = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          requirements: {
            delivered: 3,
            partial: 0,
            notCovered: 0,
            unknown: 0,
            planned: 0,
            total: 3
          }
        }
      }
    })
    expect(requirementsCardOf(all)).toMatchObject({ allDelivered: true, noData: false })
    const none = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          requirements: {
            delivered: 0,
            partial: 0,
            notCovered: 0,
            unknown: 2,
            planned: 0,
            total: 2
          }
        }
      }
    })
    expect(requirementsCardOf(none)).toMatchObject({ allDelivered: false, noData: true })
    const empty = makeReview({
      model: {
        verdict: {
          ...makeModel().verdict,
          requirements: {
            delivered: 0,
            partial: 0,
            notCovered: 0,
            unknown: 0,
            planned: 0,
            total: 0
          }
        }
      }
    })
    expect(requirementsCardOf(empty)).toMatchObject({ allDelivered: false, noData: true })
  })

  it('gate and risk: unknown and unfamiliar values never read as good', () => {
    const verdict = (gate: string, risk: string, flags: string[] = []) =>
      makeReview({
        model: { verdict: { ...makeModel().verdict, quality: { gate }, release: { risk, flags } } }
      })
    expect(gateOf(verdict('pass', 'low'))).toBe('pass')
    expect(gateOf(verdict('', 'low'))).toBe('unknown')
    expect(gateOf(verdict('flaky', 'low'))).toBe('other')
    expect(riskOf(verdict('pass', 'critical', ['migration']))).toEqual({
      risk: 'critical',
      flags: ['migration']
    })
    expect(riskOf(verdict('pass', '')).risk).toBe('unknown')
    expect(riskOf(verdict('pass', 'astronomical')).risk).toBe('other')
  })

  it('release readiness: a raised flag is present; the rest is Unknown while the risk is unknown, never ticked', () => {
    const measured = makeReview({
      model: { verdict: { ...makeModel().verdict, release: { risk: 'low', flags: ['migration'] } } }
    })
    expect(releaseReadinessOf(measured)).toEqual([
      { id: 'migration', cell: 'present' },
      { id: 'breaking', cell: 'none_found' },
      { id: 'config', cell: 'none_found' }
    ])
    const unmeasured = makeReview({
      model: { verdict: { ...makeModel().verdict, release: { risk: 'unknown', flags: [] } } }
    })
    expect(releaseReadinessOf(unmeasured).every((r) => r.cell === 'unknown')).toBe(true)
  })
})

describe('needs your decision', () => {
  it('only a delivered review has any; advisory items never ask', () => {
    const items = [
      makeItem({ key: 'a', needsDecision: true }),
      makeItem({ key: 'b', needsDecision: true, advisory: true }),
      makeItem({ key: 'c' })
    ]
    expect(needsDecisionItems(makeReview({ model: { items } })).map((i) => i.key)).toEqual(['a'])
    expect(needsDecisionItems(makeReview({ phase: 'design', model: { items } }))).toEqual([])
  })
})

describe('agentRunFactsOf', () => {
  it('sums what the runs reported and leaves the rest Unknown', () => {
    const facts = agentRunFactsOf(
      [
        makeRun(),
        makeRun({
          runId: 'u2',
          kind: 'draw',
          costActual: '0.1000 USD',
          finishedAt: '2026-10-20T10:01:00Z'
        }),
        makeRun({ runId: 'u3', status: 'failed', costActual: undefined, finishedAt: undefined })
      ],
      null
    )
    expect(facts).toMatchObject({
      reviewRuns: 3,
      reviewRunsFailed: 1,
      costUsd: 0.52,
      seconds: 180,
      tasksDone: null,
      tasksTotal: null
    })
    expect(agentRunFactsOf([], null)).toMatchObject({ costUsd: null, seconds: null, reviewRuns: 0 })
  })

  it('takes the task numbers from the execution summary', () => {
    const summary = {
      progress: {
        phasesTotal: 2,
        phasesDone: 2,
        tasksTotal: 5,
        tasksDone: 4,
        tasksFailed: 1,
        tasksRunning: 0
      }
    } as never
    expect(agentRunFactsOf([], summary)).toMatchObject({
      tasksDone: 4,
      tasksTotal: 5,
      tasksFailed: 1
    })
  })
})

describe('timeline row and hint', () => {
  it('the row word: approved and stale win over the phase', () => {
    expect(timelineRowStateOf(null)).toBe('none')
    expect(timelineRowStateOf(makeReview({ phase: 'design' }))).toBe('design')
    expect(timelineRowStateOf(makeReview())).toBe('delivered')
    expect(timelineRowStateOf(makeReview({ state: 'stale' }))).toBe('stale')
    expect(timelineRowStateOf(makeReview({ state: 'approved' }))).toBe('approved')
    expect(timelineRowStateOf(makeReview({ phase: 'weird' }))).toBe('unknown')
  })

  it('hints: design before the code, after-code once finished, decide when something waits, nothing when approved or off', () => {
    const base = {
      enabled: true,
      loaded: true,
      review: null,
      requestStatus: 'awaiting_plan_approval' as RequestStatus
    }
    expect(reviewHintOf(base)).toBe('review_design')
    expect(reviewHintOf({ ...base, requestStatus: 'executing' })).toBe('review_design')
    expect(
      reviewHintOf({ ...base, review: makeReview({ phase: 'design' }), requestStatus: 'executing' })
    ).toBe('none')
    expect(reviewHintOf({ ...base, requestStatus: 'completed' })).toBe('generate_delivered')
    expect(
      reviewHintOf({ ...base, review: makeReview({ phase: 'design' }), requestStatus: 'completed' })
    ).toBe('generate_delivered')
    const waiting = makeReview({ model: { items: [makeItem({ needsDecision: true })] } })
    expect(reviewHintOf({ ...base, review: waiting, requestStatus: 'completed' })).toBe('decide')
    expect(
      reviewHintOf({
        ...base,
        review: makeReview({
          state: 'approved',
          model: { items: [makeItem({ needsDecision: true })] }
        }),
        requestStatus: 'completed'
      })
    ).toBe('none')
    expect(reviewHintOf({ ...base, enabled: false })).toBe('none')
    expect(reviewHintOf({ ...base, loaded: false })).toBe('none')
    expect(reviewHintOf({ ...base, requestStatus: 'planning' })).toBe('none')
  })
})

describe('drawnSectionsOf', () => {
  it('names the sections the review has diagrams for, each once', () => {
    const diagrams = [
      { section: 'flow', variant: 'design', status: 'ready' },
      { section: 'flow', variant: 'delta', status: 'ready' },
      { section: 'data', variant: 'design', status: 'failed' }
    ] as never
    expect(drawnSectionsOf(makeReview({ model: makeModel({ diagrams }) }))).toEqual([
      'flow',
      'data'
    ])
  })

  it('falls back to architecture and flow when nothing is drawn yet, never to an empty list the server refuses', () => {
    expect(drawnSectionsOf(makeReview())).toEqual(['architecture', 'flow'])
  })
})
