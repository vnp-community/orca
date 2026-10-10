// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Approval, OrcaRequest, Solution } from '../../../../../shared/request-types'

// UI-STALE-SOLUTION: after a Clarification answer the service holds a NEW Solution, but the
// screen kept the old one. Same mocks as SolutionPanel.test.tsx.
const state = vi.hoisted(() => ({
  solutions: {
    solutions: [] as unknown[],
    isLoading: false,
    error: null as string | null,
    latestRun: null as unknown
  },
  approvals: { approvals: [] as unknown[] },
  choose: vi.fn(),
  approve: vi.fn(),
  refetchS: vi.fn(),
  refetchA: vi.fn()
}))

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('../../../hooks/useSolutions', () => ({
  useSolutions: () => ({
    ...state.solutions,
    generate: vi.fn(),
    generateWhenIdle: vi.fn(),
    choose: state.choose,
    refetch: state.refetchS
  })
}))
vi.mock('../../../hooks/useApprovals', () => ({
  useApprovals: () => ({
    ...state.approvals,
    approve: state.approve,
    reject: vi.fn(),
    refetch: state.refetchA,
    pendingCount: 0,
    isLoading: false,
    error: null
  })
}))
vi.mock('../../../hooks/useRequestActions', () => ({
  useRequestActions: () => ({ generatePlan: vi.fn() })
}))

import { emitRequestEvent } from '../../../lib/request-event-bus'
import { resetRequestCooldowns } from '../../../runtime/request-rate-limit'
import { RequestAnalysisTab } from './RequestAnalysisTab'

beforeEach(() => {
  state.solutions = { solutions: [], isLoading: false, error: null, latestRun: null }
  state.approvals = { approvals: [] }
  for (const f of [state.choose, state.approve, state.refetchS, state.refetchA]) {
    f.mockReset()
    f.mockResolvedValue({ ok: true, value: {} })
  }
  resetRequestCooldowns()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1',
  projectId: 'p',
  number: 1,
  title: 't',
  type: 'change_request',
  status: 'awaiting_analysis_approval',
  createdAt: '',
  updatedAt: '2026-10-10T00:00:00Z',
  ...over
})
const questions = [{ id: 'Q-1', text: 'Which database holds the audit trail?', blocking: true }]
const sol = (over: Partial<Solution> = {}): Solution => ({
  id: 's1',
  requestId: 'r1',
  kind: 'solution',
  status: 'ready',
  version: 1,
  options: [
    { id: 'o1', title: 'Alpha', summary: 'a' },
    { id: 'o2', title: 'Beta', summary: 'b' }
  ],
  ...over
})
const pendingFor = (subjectId: string): Approval =>
  ({
    id: `ap-${subjectId}`,
    requestId: 'r1',
    subjectType: 'solution',
    subjectId,
    status: 'pending',
    version: 2,
    subjectDigest: 'dg',
    createdAt: '',
    updatedAt: ''
  }) as Approval

describe('SolutionPanel keeps up with a newer Solution', () => {
  it('re-reads solutions when the Request changed state between two reads, without passing through analyzing', () => {
    state.solutions.solutions = [sol({ id: 's0', status: 'superseded', openQuestions: questions })]
    const view = render(
      <RequestAnalysisTab
        request={request({ status: 'awaiting_information' })}
        onChanged={vi.fn()}
      />
    )
    state.refetchS.mockClear()
    // Straight from awaiting_information to awaiting_analysis_approval: no `analyzing` was ever rendered.
    view.rerender(
      <RequestAnalysisTab
        request={request({ updatedAt: '2026-10-10T00:05:00Z' })}
        onChanged={vi.fn()}
      />
    )
    expect(state.refetchS).toHaveBeenCalled()
    state.solutions.solutions = [
      sol({ id: 's0', status: 'superseded' }),
      sol({ id: 's1', version: 2, openQuestions: questions })
    ]
    state.approvals.approvals = [pendingFor('s1')]
    view.rerender(
      <RequestAnalysisTab
        request={request({ updatedAt: '2026-10-10T00:05:00Z' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'ready')
    expect(screen.getByTestId('solution-blocking-question')).toHaveAttribute(
      'data-question-id',
      'Q-1'
    )
  })

  it('polls (bounded) while only a superseded version is on screen, and stops once a new one arrives', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol({ id: 's0', status: 'superseded', openQuestions: questions })]
    const onChanged = vi.fn()
    const view = render(
      <RequestAnalysisTab request={request({ status: 'analyzing' })} onChanged={onChanged} />
    )
    state.refetchS.mockClear()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    expect(onChanged).toHaveBeenCalled() // the Request is re-read too while analyzing
    expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
    state.solutions.solutions = [
      sol({ id: 's0', status: 'superseded' }),
      sol({ id: 's1', version: 2, status: 'generating' })
    ]
    view.rerender(
      <RequestAnalysisTab request={request({ status: 'analyzing' })} onChanged={onChanged} />
    )
    state.refetchS.mockClear()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(state.refetchS).not.toHaveBeenCalled()
  })

  it('gives up polling after a bounded number of tries', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol({ id: 's0', status: 'superseded' })]
    render(<RequestAnalysisTab request={request()} onChanged={vi.fn()} />)
    state.refetchS.mockClear()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000 * 80)
    })
    expect(state.refetchS).toHaveBeenCalledTimes(40)
  })

  it('does not poll for a current ready version', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol()]
    state.approvals.approvals = [pendingFor('s1')]
    render(<RequestAnalysisTab request={request()} onChanged={vi.fn()} />)
    state.refetchS.mockClear()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000)
    })
    expect(state.refetchS).not.toHaveBeenCalled()
  })

  it('re-reads on a poll-fallback event, which carries no Request id', () => {
    state.solutions.solutions = [sol()]
    render(<RequestAnalysisTab request={request()} onChanged={vi.fn()} />)
    state.refetchS.mockClear()
    act(() => emitRequestEvent({ requestId: '', eventType: 'orca.request.poll', occurredAt: '' }))
    expect(state.refetchS).toHaveBeenCalled()
  })

  it('never shows the open questions of a superseded Solution', () => {
    state.solutions.solutions = [sol({ id: 's0', status: 'superseded', openQuestions: questions })]
    render(
      <RequestAnalysisTab
        request={request({ status: 'awaiting_information' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.queryByTestId('solution-open-questions')).toBeNull()
    expect(screen.queryByTestId('solution-blocking-question')).toBeNull()
  })

  it('turns REQUEST_SOLUTION_NOT_PROPOSED into one re-read and an "updated" notice, not a raw error', async () => {
    state.solutions.solutions = [sol({ openQuestions: [] })]
    state.approvals.approvals = [pendingFor('s1')]
    state.choose.mockResolvedValue({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'runtime_error',
        message: 'REQUEST_SOLUTION_NOT_PROPOSED: solution s1 is not proposed'
      }
    })
    const onChanged = vi.fn()
    render(<RequestAnalysisTab request={request()} onChanged={onChanged} />)
    state.refetchS.mockClear()
    fireEvent.click(screen.getByTestId('solution-option-o1'))
    fireEvent.click(screen.getByTestId('solution-approve-option'))
    await waitFor(() =>
      expect(screen.getByText('This solution was just updated.')).toBeInTheDocument()
    )
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    expect(state.refetchA).toHaveBeenCalled()
    expect(onChanged).toHaveBeenCalled()
    expect(state.approve).not.toHaveBeenCalled()
    expect(screen.queryByText('This was already decided.')).toBeNull()
    expect(screen.queryByText(/not proposed/)).toBeNull()
  })
})
