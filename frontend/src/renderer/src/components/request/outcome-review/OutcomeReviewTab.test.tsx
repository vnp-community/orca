// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))

import { useAppStore } from '@/store'
import { OutcomeReviewTab } from './OutcomeReviewTab'
import { makeItem, makeModel, makeReview, makeRun, makeVerdict } from './outcome-review.fixture'
import type { UseOutcomeReviewResult } from '../../../hooks/useOutcomeReview'
import type { OrcaRequest } from '../../../../../shared/request-types'

afterEach(() => {
  cleanup()
  useAppStore.setState({ currentUser: null })
})

const asAdmin = (): void =>
  useAppStore.setState({ currentUser: { id: 'u1', role: 'admin' } as never })

const request = (status: string): OrcaRequest =>
  ({
    id: 'r1',
    number: 7,
    title: 'T',
    type: 'change_request',
    status,
    projectId: 'p1'
  }) as unknown as OrcaRequest

function hook(over: Partial<UseOutcomeReviewResult> = {}): UseOutcomeReviewResult {
  return {
    status: 'ready',
    review: makeReview(),
    estimate: null,
    runs: [],
    activeRun: null,
    error: null,
    generating: false,
    generate: vi.fn(async () => ({ ok: true as const, outcome: {} as never })),
    refresh: vi.fn(),
    ...over
  }
}

const show = (
  h: UseOutcomeReviewResult,
  status = 'completed',
  extra: Partial<Parameters<typeof OutcomeReviewTab>[0]> = {}
) => render(<OutcomeReviewTab request={request(status)} review={h} summary={null} {...extra} />)

describe('OutcomeReviewTab states', () => {
  it('loading shows a skeleton and nothing else', () => {
    show(hook({ status: 'loading', review: null }))
    expect(screen.getByTestId('outcome-review-loading')).toBeInTheDocument()
    expect(screen.queryByTestId('outcome-review-tab')).toBeNull()
  })

  it('an unavailable review says so without an error', () => {
    show(hook({ status: 'disabled', review: null }))
    expect(screen.getByTestId('outcome-review-unavailable')).toBeInTheDocument()
  })

  it('a failure shows the sentence of its code, never the raw server text, and retries', () => {
    const h = hook({
      status: 'error',
      review: null,
      error: { kind: 'unknown', code: 'REQUEST_REVIEW_NO_DEV_SERVER', message: 'RAW SERVER TEXT' }
    })
    show(h)
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('No dev server is connected for this project.')
    expect(alert).not.toHaveTextContent('RAW SERVER TEXT')
    fireEvent.click(within(alert).getByRole('button', { name: 'Retry' }))
    expect(h.refresh).toHaveBeenCalled()
    cleanup()
    show(
      hook({
        status: 'error',
        review: null,
        error: { kind: 'unknown', code: 'REQUEST_REVIEW_SOMETHING_NEW', message: 'x' }
      })
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      'The review could not be loaded. Try again.'
    )
  })

  it('before the code, with no review: the invitation with what asking would do, and a facts-only choice', () => {
    const h = hook({
      status: 'none',
      review: null,
      estimate: {
        phase: 'design',
        executionFinished: false,
        reviewExists: false,
        stale: false,
        currentInputsDigest: 'd',
        warnings: [],
        contextCached: false,
        designLocked: false,
        contextRunnable: true,
        contextGaps: []
      }
    })
    show(h, 'awaiting_plan_approval')
    const invite = screen.getByTestId('outcome-review-invite')
    expect(invite).toHaveAttribute('data-kind', 'design')
    expect(invite).toHaveTextContent('Reading the design takes one agent run.')
    expect(invite).toHaveTextContent('No cost figures have been measured yet.')
    fireEvent.click(screen.getByTestId('outcome-review-generate'))
    expect(h.generate).toHaveBeenCalledWith({ runContext: true })
    fireEvent.click(screen.getByTestId('outcome-review-facts-only'))
    expect(h.generate).toHaveBeenLastCalledWith({ runContext: false })
  })

  it('an estimate that says the run cannot start names why, in words', () => {
    const h = hook({
      status: 'none',
      review: null,
      estimate: {
        phase: 'design',
        executionFinished: false,
        reviewExists: false,
        stale: false,
        currentInputsDigest: 'd',
        warnings: [],
        contextCached: false,
        designLocked: false,
        contextRunnable: false,
        contextGaps: ['no_dev_server', 'REQUEST_REVIEW_SKILL_MISSING']
      }
    })
    show(h, 'executing')
    const text = screen.getByTestId('outcome-review-estimate').textContent ?? ''
    expect(text).toContain('no dev server is connected')
    expect(text).toContain('A skill the review needs is not installed on the dev server.')
    expect(text).not.toContain('no_dev_server')
  })

  it('a cached estimate promises no new agent run', () => {
    show(
      hook({
        status: 'none',
        review: null,
        estimate: {
          phase: 'delivered',
          executionFinished: true,
          reviewExists: false,
          stale: false,
          currentInputsDigest: 'd',
          warnings: [],
          contextCached: true,
          designLocked: true,
          contextRunnable: true,
          contextGaps: []
        }
      }),
      'completed'
    )
    expect(screen.getByTestId('outcome-review-invite')).toHaveAttribute('data-kind', 'delivered')
    expect(screen.getByTestId('outcome-review-estimate')).toHaveTextContent(
      'A checked result is kept, so no new agent run is needed.'
    )
  })

  it('design phase: the design alone, a banner, no verdict, no decision list, no release blocks', () => {
    show(
      hook({
        review: makeReview({
          phase: 'design',
          model: { items: [makeItem({ section: 'architecture' })] }
        })
      }),
      'executing'
    )
    expect(screen.getByTestId('outcome-review-tab')).toHaveAttribute('data-phase', 'design')
    expect(screen.getByTestId('outcome-banner-design_phase')).toHaveTextContent(
      'The after-code view appears when execution completes.'
    )
    expect(screen.getByTestId('outcome-design-card-architecture')).toHaveAttribute(
      'data-mode',
      'design'
    )
    expect(screen.getByTestId('outcome-needs-decision-later')).toHaveTextContent(
      'Decisions open after execution completes.'
    )
    for (const id of [
      'outcome-verdict-cards',
      'outcome-release-readiness',
      'outcome-agent-run-facts',
      'outcome-needs-decision'
    ]) {
      expect(screen.queryByTestId(id)).toBeNull()
    }
  })

  it('a design review once Execution is over offers the after-code review right in its banner', () => {
    asAdmin()
    const h = hook({ review: makeReview({ phase: 'design' }) })
    show(h, 'completed')
    fireEvent.click(screen.getByTestId('outcome-banner-generate'))
    expect(h.generate).toHaveBeenCalledWith({ runContext: true })
  })

  it('delivered: the design control first, then the verdicts, then what needs a decision', () => {
    const items = [
      makeItem({ key: 'k1', needsDecision: true, title: 'Gateway calls bug management' }),
      makeItem({
        key: 'k2',
        needsDecision: true,
        advisory: true,
        title: 'Rests on an unapproved element'
      })
    ]
    show(hook({ review: makeReview({ model: { items } }) }))
    const order = [
      'outcome-design-control',
      'outcome-verdict-cards',
      'outcome-needs-decision',
      'outcome-release-readiness',
      'outcome-agent-run-facts'
    ].map((id) => screen.getByTestId(id))
    for (let i = 1; i < order.length; i++) {
      expect(
        order[i - 1].compareDocumentPosition(order[i]) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
    }
    const decisionList = screen.getByTestId('outcome-needs-decision')
    expect(within(decisionList).getAllByTestId('outcome-decision-item')).toHaveLength(1)
    expect(decisionList).toHaveTextContent('Gateway calls bug management')
    expect(decisionList).not.toHaveTextContent('Rests on an unapproved element')
    expect(within(decisionList).getByText('Agent reported')).toBeInTheDocument()
  })

  it('without a design to compare with the not-in-design counts read Unknown, never 0', () => {
    show(
      hook({
        review: makeReview({
          model: {
            verdict: {
              ...makeModel().verdict,
              architecture: makeVerdict({ designChecked: false }),
              flow: makeVerdict({ designChecked: false })
            }
          }
        })
      })
    )
    expect(screen.getByTestId('outcome-design-row-architecture-unplanned_edges')).toHaveTextContent(
      'Unknown'
    )
    expect(screen.getByTestId('outcome-design-row-flow-unplanned_calls')).toHaveTextContent(
      'Unknown'
    )
    expect(screen.getByTestId('outcome-design-source-architecture')).toHaveTextContent(
      'Design not available'
    )
    expect(screen.getByTestId('outcome-verdict-architecture')).toHaveTextContent('Unknown')
  })

  it('a graph that was not available hides the snapshot and says the checks are limited', () => {
    show(
      hook({
        review: makeReview({
          model: { kg: { available: false, snapshot: 'snap-SECRET', gaps: [], conflicts: [] } }
        })
      })
    )
    expect(screen.getByTestId('outcome-banner-kg_unavailable')).toHaveTextContent(
      'architecture checks are limited'
    )
    expect(screen.getByTestId('outcome-review-kg')).toHaveTextContent('KG unavailable')
    expect(document.body.textContent).not.toContain('snap-SECRET')
  })

  it('a low share of approved design elements raises the advisory banner with the percentage', () => {
    show(
      hook({
        review: makeReview({
          model: {
            verdict: {
              ...makeModel().verdict,
              architecture: makeVerdict({ designChecked: true, reviewedRatio: 0.4 })
            }
          }
        })
      })
    )
    expect(screen.getByTestId('outcome-banner-design_not_reviewed')).toHaveTextContent(
      'Design not reviewed: 40% of the design elements read are approved. Findings based on them are advisory.'
    )
  })

  it('stale: the warning, a way to build the review again for those who may, and the old content still there', () => {
    asAdmin()
    const h = hook({ review: makeReview({ state: 'stale' }) })
    show(h)
    const banner = screen.getByTestId('outcome-banner-stale')
    expect(banner).toHaveTextContent('This review was made before the Request changed.')
    fireEvent.click(within(banner).getByRole('button', { name: 'Rebuild the review' }))
    expect(h.generate).toHaveBeenCalledWith({ runContext: true })
    expect(screen.getByTestId('outcome-design-control')).toBeInTheDocument()
  })

  it('someone who may not act sees the warning but no button that would be refused', () => {
    show(hook({ review: makeReview({ state: 'stale' }) }))
    expect(screen.getByTestId('outcome-banner-stale')).toBeInTheDocument()
    expect(screen.queryByTestId('outcome-banner-rebuild')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Regenerate…' })).toBeNull()
  })

  it('a run that is going shows a skeleton for the agent part, keeps the facts, and announces progress politely', () => {
    show(
      hook({
        activeRun: makeRun({ status: 'running', finishedAt: undefined }),
        runs: [makeRun({ status: 'running', finishedAt: undefined })]
      })
    )
    expect(screen.getByTestId('outcome-review-pending-block')).toBeInTheDocument()
    expect(screen.queryByTestId('outcome-design-control')).toBeNull()
    expect(screen.getByTestId('outcome-verdict-cards')).toBeInTheDocument()
    const live = screen.getByTestId('outcome-review-progress')
    expect(live).toHaveAttribute('aria-live', 'polite')
    expect(live).toHaveTextContent('Reading the design and the code…')
  })

  it('the progress region stays mounted while empty so a screen reader hears what appears in it', () => {
    show(hook())
    expect(screen.getByTestId('outcome-review-progress')).toHaveAttribute('aria-live', 'polite')
    expect(screen.getByTestId('outcome-review-progress')).toBeEmptyDOMElement()
  })

  it('a failed run explains itself with the sentence of its code and keeps the facts on screen', () => {
    show(
      hook({ runs: [makeRun({ status: 'failed', errorCode: 'REQUEST_REVIEW_RESULT_INVALID' })] })
    )
    expect(screen.getByTestId('outcome-banner-run_failed')).toHaveTextContent(
      "The agent's answer could not be used. Try again."
    )
    expect(screen.getByTestId('outcome-verdict-cards')).toBeInTheDocument()
  })

  it('a drawing that failed says the table is shown instead', () => {
    show(
      hook({
        review: makeReview({
          model: {
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
      })
    )
    expect(screen.getByTestId('outcome-banner-drawing_failed')).toHaveTextContent(
      'Drawing failed, showing the table.'
    )
  })

  it('requirements: nothing to conclude from is Unknown, a full set only says the tied tasks are done', () => {
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
    show(hook({ review: none }))
    expect(screen.getByTestId('outcome-verdict-requirements')).toHaveTextContent(
      'Not enough data to conclude'
    )
    cleanup()
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
    show(hook({ review: all }))
    expect(screen.getByTestId('outcome-verdict-requirements')).toHaveTextContent('3 of 3')
    expect(screen.getByTestId('outcome-verdict-requirements')).toHaveTextContent(
      'Every task tied to these criteria is done'
    )
  })

  it('release readiness never ticks a box it could not measure', () => {
    show(
      hook({
        review: makeReview({
          model: {
            verdict: {
              ...makeModel().verdict,
              release: { risk: 'unknown', flags: [] },
              quality: { gate: 'unknown' }
            }
          }
        })
      })
    )
    for (const id of ['migration', 'breaking', 'config']) {
      expect(screen.getByTestId(`outcome-readiness-${id}`)).toHaveAttribute('data-cell', 'unknown')
    }
    expect(screen.getByTestId('outcome-verdict-quality')).toHaveTextContent(
      'Not enough data to conclude'
    )
    expect(screen.getByTestId('outcome-verdict-release')).toHaveTextContent(
      'Risk could not be measured'
    )
  })

  it('an approved review is labelled Approved in the header, nothing more', () => {
    show(hook({ review: makeReview({ state: 'approved', approvedBy: 'u1' }) }))
    expect(screen.getByTestId('outcome-review-state')).toHaveTextContent('Approved')
  })
})
