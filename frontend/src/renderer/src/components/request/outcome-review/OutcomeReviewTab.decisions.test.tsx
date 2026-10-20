// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))
const rpc = vi.hoisted(() => vi.fn())
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))
const runtimeRpc = vi.hoisted(() => vi.fn())
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...a: unknown[]) => runtimeRpc(...a),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
const openReview = vi.hoisted(() => vi.fn(() => true))
vi.mock('../../review-map/entry/open-review-entry', () => ({
  openReviewFromEntryPoint: openReview
}))

import { useAppStore } from '@/store'
import { resetChildRequestFetches } from '../../../hooks/useChildRequestStatuses'
import { resetReviewArtifactCache } from '../../../hooks/useReviewArtifact'
import { OutcomeReviewTab } from './OutcomeReviewTab'
import { makeItem, makeReview } from './outcome-review.fixture'
import type { UseOutcomeReviewResult } from '../../../hooks/useOutcomeReview'
import type { OrcaRequest } from '../../../../../shared/request-types'

const request = (status = 'completed', over: Record<string, unknown> = {}): OrcaRequest =>
  ({
    id: 'r1',
    number: 7,
    title: 'Add review',
    type: 'change_request',
    status,
    projectId: 'p1',
    planTaskId: 'plan-1',
    reporterId: 'u1',
    ...over
  }) as unknown as OrcaRequest

const items = [
  makeItem({
    key: 'a',
    title: 'Gateway calls bug management',
    needsDecision: true,
    origin: 'agent',
    section: 'architecture',
    containers: ['ms-gateway']
  }),
  makeItem({
    key: 'b',
    title: 'Old call removed',
    needsDecision: true,
    origin: 'fact',
    section: 'flow',
    change: 'removed'
  })
]
const hook = (over: Partial<UseOutcomeReviewResult> = {}): UseOutcomeReviewResult => ({
  status: 'ready',
  review: makeReview({ version: 5, model: { items } }),
  estimate: null,
  runs: [],
  activeRun: null,
  error: null,
  generating: false,
  generate: vi.fn(async () => ({ ok: true as const, outcome: {} as never })),
  refresh: vi.fn(),
  ...over
})

let decisions: Record<string, unknown>[] = []
const dec = (itemKey: string, over: Record<string, unknown> = {}) => ({
  decisionId: `d-${itemKey}`,
  reviewId: 'v1',
  itemKey,
  decision: 'accept_exception',
  note: 'ok',
  decidedBy: 'u1',
  decidedAt: '',
  active: true,
  orphaned: false,
  ...over
})

beforeEach(() => {
  rpc.mockReset()
  runtimeRpc.mockReset()
  openReview.mockClear()
  resetChildRequestFetches()
  resetReviewArtifactCache()
  decisions = []
  useAppStore.setState({
    currentUser: { id: 'u1', name: 'Ada', role: 'developer' } as never,
    codeIntelSupportState: { state: 'enabled' } as never,
    requestsById: {}
  })
  runtimeRpc.mockResolvedValue({
    tasks: [
      { id: 't1', requestId: 'r1', worktreeId: 'wt-1', title: 'one', status: 'done', type: 'task' }
    ]
  })
  rpc.mockImplementation(async (method: string, p: Record<string, unknown>) => {
    switch (method) {
      case 'request.review.decisions':
        return { ok: true, value: { decisions, openDecisions: 0 } }
      case 'request.review.decide':
        decisions = [
          ...decisions.filter((d) => d.itemKey !== p.itemKey),
          dec(String(p.itemKey), { decision: p.decision, note: p.note })
        ]
        return {
          ok: true,
          value: {
            decision: dec(String(p.itemKey)),
            reviewVersion: Number(p.expectedVersion) + 1,
            reviewState: 'ready',
            openDecisions: 0
          }
        }
      case 'request.review.approve':
        return { ok: true, value: { review: { reviewId: 'v1', version: 9, state: 'approved' } } }
      default:
        return { ok: false, error: { kind: 'not_found' } }
    }
  })
})
afterEach(() => {
  cleanup()
  useAppStore.setState({ currentUser: null })
})

const show = (h = hook(), r = request()) =>
  render(<OutcomeReviewTab request={r} review={h} summary={null} />)

describe('OutcomeReviewTab decisions', () => {
  it('the reporter sees a card for each item that asks for a decision, and the bar under the other blocks', async () => {
    show()
    expect(await screen.findAllByTestId('outcome-decision-card')).toHaveLength(2)
    const bar = screen.getByTestId('outcome-decision-bar')
    const facts = screen.getByTestId('outcome-agent-run-facts')
    expect(facts.compareDocumentPosition(bar) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('someone who may not act sees what asks for a decision but has no controls', () => {
    useAppStore.setState({ currentUser: { id: 'someone', name: 'Bo', role: 'developer' } as never })
    show()
    expect(screen.getAllByTestId('outcome-decision-item')).toHaveLength(2)
    expect(screen.queryByTestId('outcome-decision-card')).toBeNull()
    expect(screen.queryByTestId('outcome-decision-bar')).toBeNull()
  })

  it('before the code there is nothing to decide: no cards, no bar, and no call to the decision channels', () => {
    show(hook({ review: makeReview({ phase: 'design', model: { items } }) }), request('executing'))
    expect(screen.getByTestId('outcome-needs-decision-later')).toBeInTheDocument()
    expect(screen.queryByTestId('outcome-decision-card')).toBeNull()
    expect(screen.queryByTestId('outcome-decision-bar')).toBeNull()
    expect(rpc.mock.calls.map((c) => c[0])).not.toContain('request.review.decisions')
  })

  it('Approve with decisions open says so by the button and takes the viewer to the first open item; nothing is sent', async () => {
    show()
    await screen.findAllByTestId('outcome-decision-card')
    const first = screen.getAllByTestId('outcome-decision-item')[0]
    const scroll = vi.fn()
    first.querySelector('[data-testid="outcome-decision-card"]')!.parentElement!.scrollIntoView =
      scroll
    fireEvent.click(screen.getByTestId('outcome-approve'))
    expect(screen.getByTestId('outcome-approve-error')).toHaveTextContent(
      'Resolve 2 open decisions first'
    )
    expect(scroll).toHaveBeenCalled()
    expect(rpc.mock.calls.map((c) => c[0])).not.toContain('request.review.approve')
  })

  it('after each item is decided, Approve goes to the server with the version of the review', async () => {
    const h = hook()
    show(h)
    const cards = await screen.findAllByTestId('outcome-decision-card')
    fireEvent.click(within(cards[0]).getByRole('button', { name: 'Send back' }))
    await waitFor(() => expect(screen.getAllByTestId('outcome-decision-made')).toHaveLength(1))
    fireEvent.click(
      within(screen.getAllByTestId('outcome-decision-card')[1]).getByRole('button', {
        name: 'Send back'
      })
    )
    await waitFor(() => expect(screen.getAllByTestId('outcome-decision-made')).toHaveLength(2))
    expect(h.refresh).toHaveBeenCalled()
    fireEvent.click(screen.getByTestId('outcome-approve'))
    await waitFor(() => expect(rpc.mock.calls.map((c) => c[0])).toContain('request.review.approve'))
    const approve = rpc.mock.calls.find((c) => c[0] === 'request.review.approve')!
    expect(approve[1]).toMatchObject({ id: 'r1', reviewId: 'v1', expectedVersion: 7 }) // 5 + one per decision
    expect(screen.getByTestId('outcome-request-changes')).toHaveTextContent('Request changes (2)')
  })

  it('a finished follow-up is announced, and nothing regenerates by itself', async () => {
    decisions = [dec('a', { decision: 'send_back', childRequestId: 'child-1' })]
    useAppStore.setState({
      requestsById: {
        'child-1': {
          id: 'child-1',
          number: 8,
          title: 'x',
          type: 'bug',
          status: 'completed',
          projectId: 'p1'
        } as never
      }
    })
    const h = hook()
    show(h)
    expect(await screen.findByTestId('outcome-follow-up-finished')).toHaveTextContent(
      'A follow-up Request finished. Regenerate this review to check.'
    )
    expect(h.generate).not.toHaveBeenCalled()
    expect(rpc.mock.calls.map((c) => c[0])).not.toContain('request.review.regenerate')
  })

  it('a follow-up that has not finished raises no banner', async () => {
    decisions = [dec('a', { decision: 'send_back', childRequestId: 'child-1' })]
    useAppStore.setState({
      requestsById: {
        'child-1': {
          id: 'child-1',
          number: 8,
          title: 'x',
          type: 'bug',
          status: 'executing',
          projectId: 'p1'
        } as never
      }
    })
    show()
    await screen.findAllByTestId('outcome-decision-card')
    expect(screen.queryByTestId('outcome-follow-up-finished')).toBeNull()
  })

  it('"Open in code review" opens the lens of the row in the worktree of the task the review came from', async () => {
    const review = makeReview({
      model: {
        items,
        subject: { baseRef: 'main', heads: [{ branch: 'feat/one', commit: 'abc', taskId: 't1' }] }
      }
    })
    show(hook({ review }))
    await waitFor(() =>
      expect(screen.getAllByTestId('outcome-code-review')[0]).toHaveAttribute('data-state', 'ready')
    )
    const row = screen.getByTestId('outcome-diff-row')
    fireEvent.click(within(row).getByRole('button', { name: 'Open in code review' }))
    expect(openReview).toHaveBeenCalledWith('wt-1', 'request-outcome', { lens: 'architecture' })
    fireEvent.click(
      within(screen.getByTestId('outcome-container-drilldown')).getByRole('button', {
        name: 'Open in code review'
      })
    )
    expect(openReview).toHaveBeenLastCalledWith('wt-1', 'request-outcome', { lens: 'architecture' })
  })

  it('with no worktree to open the buttons say so instead of doing nothing', async () => {
    runtimeRpc.mockResolvedValue({
      tasks: [{ id: 't1', requestId: 'r1', title: 'one', status: 'done', type: 'task' }]
    })
    const review = makeReview({
      model: {
        items,
        subject: { baseRef: 'main', heads: [{ branch: 'feat/one', commit: 'abc', taskId: 't1' }] }
      }
    })
    show(hook({ review }))
    await waitFor(() =>
      expect(screen.getAllByTestId('outcome-code-review')[0]).toHaveAttribute(
        'data-state',
        'no_worktree'
      )
    )
    expect(screen.getAllByTestId('outcome-code-review')[0]).toHaveAttribute(
      'title',
      expect.stringContaining('not available for this Request')
    )
  })

  it('with the code review off the buttons say so', async () => {
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled' } as never })
    show()
    expect((await screen.findAllByTestId('outcome-code-review'))[0]).toHaveAttribute(
      'data-state',
      'unsupported'
    )
  })
})
