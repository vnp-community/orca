// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useRequestMock = vi.fn()
vi.mock('../../hooks/useRequest', () => ({ useRequest: (...a: unknown[]) => useRequestMock(...a) }))
vi.mock('../../hooks/useRequestActions', () => ({ useRequestActions: () => ({ reopen: vi.fn() }) }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))

const rpc = vi.fn()
vi.mock('../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))

import { resetOutcomeReviewFlagCache } from '../../hooks/useOutcomeReviewEnabled'
import { RequestDetailPane } from './RequestDetailPane'

const request = (status: string) => ({
  id: 'r1',
  number: 7,
  title: 'Add review',
  type: 'change_request',
  status,
  projectId: 'p1',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z'
})

type Replies = { enabled: boolean; review?: unknown }
function script({ enabled, review }: Replies): void {
  rpc.mockImplementation(async (method: string) => {
    switch (method) {
      case 'request.engineSettings':
        return {
          ok: true,
          value: {
            projectId: 'p1',
            engine: 'native',
            executionMode: 'step_by_step',
            outcomeReviewEnabled: enabled,
            version: 1
          }
        }
      case 'request.review.get':
        return { ok: true, value: review ? { found: true, review } : { found: false } }
      case 'request.review.estimate':
        return {
          ok: true,
          value: {
            phase: 'design',
            currentInputsDigest: 'd',
            contextRunnable: true,
            contextGaps: [],
            warnings: []
          }
        }
      case 'request.review.runs':
        return { ok: true, value: { runs: [] } }
      case 'request.review.generate':
        return {
          ok: false,
          error: {
            kind: 'invalid_state',
            backendCode: 'REQUEST_REVIEW_SKILL_MISSING',
            message: 'm'
          }
        }
      default:
        return { ok: false, error: { kind: 'not_found' } }
    }
  })
}

const reviewCalls = (): string[] =>
  rpc.mock.calls.map((c) => c[0] as string).filter((m) => m.startsWith('request.review.'))

beforeEach(() => {
  rpc.mockReset()
  useRequestMock.mockReset()
  resetOutcomeReviewFlagCache()
  useRequestMock.mockImplementation(() => ({
    request: request('executing'),
    history: [],
    links: [],
    linksSupported: true,
    isLoading: false,
    error: null,
    refetch: vi.fn()
  }))
})
afterEach(cleanup)

describe('RequestDetailPane and the Outcome review', () => {
  it('with the review off there is no tab, no timeline row and no review traffic at all', async () => {
    script({ enabled: false })
    render(<RequestDetailPane requestId="r1" />)
    await waitFor(() =>
      expect(rpc).toHaveBeenCalledWith('request.engineSettings', { projectId: 'p1' })
    )
    expect(screen.queryByRole('tab', { name: 'Outcome review' })).toBeNull()
    expect(document.querySelector('[data-step-id="review"]')).toBeNull()
    expect(reviewCalls()).toEqual([])
  })

  it('with the review on the tab comes right after Execution, and opening it only reads', async () => {
    script({ enabled: true })
    render(<RequestDetailPane requestId="r1" />)
    const tab = await screen.findByRole('tab', { name: 'Outcome review' })
    const names = screen.getAllByRole('tab').map((t) => t.textContent)
    const execution = names.findIndex((n) => n?.startsWith('Execution'))
    expect(execution).toBeGreaterThan(-1)
    expect(names[execution + 1]).toBe('Outcome review')
    expect(names.slice(execution + 2)).toEqual(['History', 'Related'])
    expect(document.querySelector('[data-step-id="review"]')).not.toBeNull()

    fireEvent.mouseDown(tab)
    fireEvent.click(tab)
    expect(await screen.findByTestId('outcome-review-invite')).toBeInTheDocument()
    expect(screen.getByTestId('outcome-review-invite')).toHaveAttribute('data-kind', 'design')
    expect(reviewCalls()).toContain('request.review.get')
    expect(reviewCalls()).toContain('request.review.estimate')
    expect(reviewCalls()).not.toContain('request.review.generate')
  })

  it('pressing Generate asks for a review once and says why when the server refuses', async () => {
    script({ enabled: true })
    render(<RequestDetailPane requestId="r1" />)
    const tab = await screen.findByRole('tab', { name: 'Outcome review' })
    fireEvent.mouseDown(tab)
    fireEvent.click(tab)
    fireEvent.click(await screen.findByTestId('outcome-review-generate'))
    await waitFor(() =>
      expect(reviewCalls().filter((m) => m === 'request.review.generate')).toHaveLength(1)
    )
    const call = rpc.mock.calls.find((c) => c[0] === 'request.review.generate')
    expect(call?.[1]).toEqual({ id: 'r1', runContext: true })
  })

  it('a Request that has no plan yet has no tab and sends nothing, even with the review on', async () => {
    useRequestMock.mockImplementation(() => ({
      request: request('analyzing'),
      history: [],
      links: [],
      linksSupported: true,
      isLoading: false,
      error: null,
      refetch: vi.fn()
    }))
    script({ enabled: true })
    render(<RequestDetailPane requestId="r1" />)
    await waitFor(() =>
      expect(rpc).toHaveBeenCalledWith('request.engineSettings', { projectId: 'p1' })
    )
    expect(screen.queryByRole('tab', { name: 'Outcome review' })).toBeNull()
    expect(reviewCalls()).toEqual([])
  })

  it('a completed Request with no review is invited to generate the after-code one', async () => {
    useRequestMock.mockImplementation(() => ({
      request: request('completed'),
      history: [],
      links: [],
      linksSupported: true,
      isLoading: false,
      error: null,
      refetch: vi.fn()
    }))
    script({ enabled: true })
    render(<RequestDetailPane requestId="r1" />)
    const tab = await screen.findByRole('tab', { name: 'Outcome review' })
    fireEvent.mouseDown(tab)
    fireEvent.click(tab)
    expect(await screen.findByTestId('outcome-review-invite')).toHaveAttribute(
      'data-kind',
      'delivered'
    )
    const bar = await screen.findByTestId('next-action-bar')
    expect(bar).toHaveTextContent('Generate after-code review')
  })
})
