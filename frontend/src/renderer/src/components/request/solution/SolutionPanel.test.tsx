// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Approval, OrcaRequest, Solution } from '../../../../../shared/request-types'

const state = vi.hoisted(() => ({
  solutions: {
    solutions: [] as unknown[],
    isLoading: false,
    error: null as string | null,
    latestRun: null as unknown
  },
  approvals: { approvals: [] as unknown[] },
  choose: vi.fn(),
  generate: vi.fn(),
  generateIdle: vi.fn(),
  approve: vi.fn(),
  reject: vi.fn(),
  refetchS: vi.fn(),
  refetchA: vi.fn(),
  generatePlan: vi.fn()
}))

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('../../../hooks/useSolutions', () => ({
  useSolutions: () => ({
    ...state.solutions,
    generate: state.generate,
    generateWhenIdle: state.generateIdle,
    choose: state.choose,
    refetch: state.refetchS
  })
}))
vi.mock('../../../hooks/useApprovals', () => ({
  useApprovals: () => ({
    ...state.approvals,
    approve: state.approve,
    reject: state.reject,
    refetch: state.refetchA,
    pendingCount: 0,
    isLoading: false,
    error: null
  })
}))
vi.mock('../../../hooks/useRequestActions', () => ({
  useRequestActions: () => ({ generatePlan: state.generatePlan })
}))

import { useAppStore } from '@/store'
import { resetRequestCooldowns, startRequestCooldown } from '../../../runtime/request-rate-limit'
import { RequestAnalysisTab } from './RequestAnalysisTab'
import { emitRequestEvent } from '../../../lib/request-event-bus'

afterEach(cleanup)
beforeEach(() => {
  state.solutions = { solutions: [], isLoading: false, error: null, latestRun: null }
  state.approvals = { approvals: [] }
  for (const f of [
    state.choose,
    state.generate,
    state.generateIdle,
    state.approve,
    state.reject,
    state.refetchS,
    state.refetchA,
    state.generatePlan
  ]) {
    f.mockReset()
    f.mockResolvedValue({ ok: true, value: {} })
  }
  state.generateIdle.mockResolvedValue({ ok: true, value: { skipped: false } })
})

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1',
  projectId: 'p',
  number: 1,
  title: 't',
  type: 'change_request',
  status: 'awaiting_analysis_approval',
  createdAt: '',
  updatedAt: '',
  ...over
})
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
const pending = {
  id: 'ap1',
  requestId: 'r1',
  subjectType: 'solution',
  subjectId: 's1',
  status: 'pending',
  version: 2,
  subjectDigest: 'dg',
  createdAt: '',
  updatedAt: ''
} as Approval

const mount = (r = request(), onChanged = vi.fn()) =>
  render(<RequestAnalysisTab request={r} onChanged={onChanged} />)

describe('RequestAnalysisTab / SolutionPanel', () => {
  it('renders nothing for types without analysis', () => {
    const { container } = mount(request({ type: 'task' }))
    expect(container).toBeEmptyDOMElement()
  })

  it('shows skeleton while loading, retry on network error, message on forbidden', () => {
    state.solutions.isLoading = true
    const { unmount } = mount()
    expect(screen.getByTestId('solution-panel-loading')).toBeInTheDocument()
    unmount()
    state.solutions = { solutions: [], isLoading: false, error: 'network', latestRun: null }
    const second = mount()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(state.refetchS).toHaveBeenCalled()
    second.unmount()
    state.solutions.error = 'forbidden'
    mount()
    expect(screen.getByRole('alert')).toHaveTextContent('permission to view')
  })

  it('shows the analyzing state while an AI run is in flight', () => {
    state.solutions.latestRun = { id: 'run1', kind: 'solution', status: 'running' }
    mount(request({ status: 'analyzing' }))
    expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
    expect(screen.queryByTestId('solution-generate')).toBeNull()
  })

  it.each([
    ['diagnosis', 'diagnosis-view'],
    ['findings', 'findings-view'],
    ['answer', 'answer-view']
  ] as const)('renders %s view with missing fields', (kind, testId) => {
    state.solutions.solutions = [sol({ kind, options: undefined, content: '{"unrelated": 1}' })]
    mount(request({ type: 'bug' }))
    expect(screen.getByTestId(testId)).toBeInTheDocument()
  })

  it('renders structured diagnosis sections', () => {
    state.solutions.solutions = [
      sol({
        kind: 'diagnosis',
        options: undefined,
        content: JSON.stringify({ root_cause: 'Null deref' })
      })
    ]
    mount(request({ type: 'bug' }))
    expect(screen.getByText('Null deref')).toBeInTheDocument()
    expect(screen.getAllByText('No data').length).toBe(2)
  })

  it('shows drafting banner for generating status', () => {
    state.solutions.solutions = [sol({ status: 'generating' })]
    mount()
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute(
      'data-status',
      'generating'
    )
    expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
  })

  it('collapses superseded versions with a "Superseded" badge and is read-only', () => {
    state.solutions.solutions = [
      sol({ id: 's0', status: 'superseded', version: 1 }),
      sol({ id: 's1', version: 2 })
    ]
    mount()
    expect(screen.getByRole('button', { name: /Superseded/ })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Superseded/ }))
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute(
      'data-status',
      'superseded'
    )
    expect(screen.queryByTestId('solution-decision-bar')).toBeNull()
  })

  it('chooses then approves in order with approval version and digest', async () => {
    state.solutions.solutions = [sol()]
    state.approvals.approvals = [pending]
    state.choose.mockResolvedValue({ ok: true, value: { approvalDigest: 'fresh' } })
    const onChanged = vi.fn()
    mount(request(), onChanged)
    const approveBtn = screen.getByRole('button', { name: 'Approve this option' })
    expect(approveBtn).toBeDisabled()
    fireEvent.click(screen.getByTestId('solution-option-o2'))
    fireEvent.click(approveBtn)
    await waitFor(() => expect(state.approve).toHaveBeenCalled())
    expect(state.choose).toHaveBeenCalledWith({ solutionId: 's1', optionId: 'o2' })
    expect(state.choose.mock.invocationCallOrder[0]).toBeLessThan(
      state.approve.mock.invocationCallOrder[0]
    )
    expect(state.approve.mock.calls[0][0].approval).toMatchObject({
      id: 'ap1',
      version: 2,
      subjectDigest: 'fresh'
    })
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
  })

  it('reject dialog sends approval.reject with the comment', async () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [pending]
    mount(request({ type: 'bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'too short' } })
    expect(
      screen.getByRole('dialog').querySelector('button[data-variant="destructive"]')
    ).toBeDisabled()
    fireEvent.change(box, { target: { value: 'this is a valid reason' } })
    fireEvent.click(screen.getAllByRole('button', { name: 'Reject' }).at(-1)!)
    await waitFor(() => expect(state.reject).toHaveBeenCalled())
    expect(state.reject.mock.calls[0][0]).toMatchObject({ comment: 'this is a valid reason' })
    await waitFor(() => expect(state.generate).toHaveBeenCalled())
  })

  it('answer uses the Accept label; hotfix has no decision bar', () => {
    state.solutions.solutions = [sol({ kind: 'answer', options: undefined })]
    state.approvals.approvals = [pending]
    const { unmount } = mount(request({ type: 'question' }))
    expect(screen.getByRole('button', { name: 'Accept' })).toBeInTheDocument()
    unmount()
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    mount(request({ type: 'hotfix' }))
    expect(screen.queryByTestId('solution-decision-bar')).toBeNull()
  })

  it('shows no-permission bar after APPROVAL_NOT_APPROVER', async () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [pending]
    state.approve.mockResolvedValue({
      ok: false,
      error: { kind: 'unknown', code: 'x', message: 'APPROVAL_NOT_APPROVER: no' }
    })
    mount(request({ type: 'bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    expect(await screen.findByText('You do not have permission to approve')).toBeInTheDocument()
  })

  it('conflict refetches and shows the updated banner', async () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [pending]
    state.approve.mockResolvedValue({
      ok: false,
      error: { kind: 'conflict', code: 'x', message: 'APPROVAL_VERSION_CONFLICT: stale' }
    })
    mount(request({ type: 'bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    expect(await screen.findByText('This solution was just updated.')).toBeInTheDocument()
    expect(state.refetchS).toHaveBeenCalled()
  })

  it('expired approval only allows regenerate', () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [{ ...pending, status: 'expired' }]
    mount(request({ type: 'bug' }))
    expect(screen.getByText('Expired')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeInTheDocument()
  })

  it('refetches on solution and approval events for this request only', () => {
    state.solutions.solutions = [sol()]
    mount()
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.solution.proposed',
      occurredAt: ''
    })
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    emitRequestEvent({
      requestId: 'other',
      eventType: 'orca.request.solution.proposed',
      occurredAt: ''
    })
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.request.classified',
      occurredAt: ''
    })
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    emitRequestEvent({ requestId: 'r1', eventType: 'approval.decided', occurredAt: '' })
    expect(state.refetchA).toHaveBeenCalledTimes(2)
  })

  it('is read-only after the analysis stage', () => {
    state.solutions.solutions = [sol({ status: 'chosen', chosenOptionId: 'o1' })]
    mount(request({ status: 'planning' }))
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'chosen')
    expect(screen.queryByTestId('solution-decision-bar')).toBeNull()
    expect(screen.getByText('Chosen')).toBeInTheDocument()
  })
})

describe('failed AI runs (TASK-FE-E2E-005)', () => {
  beforeEach(() => {
    resetRequestCooldowns()
    useAppStore.setState({ currentUser: { id: 'u1', role: 'member' } } as never)
  })

  it('shows the AI setup banner with the fix when the only run failed for lack of AI', () => {
    state.solutions.latestRun = {
      id: 'run1',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_AI_NOT_CONFIGURED'
    }
    mount(request({ status: 'analyzing' }))
    const banner = screen.getByTestId('request-ai-setup-banner')
    expect(banner).toHaveAttribute('data-ai-code', 'REQUEST_AI_NOT_CONFIGURED')
    expect(banner.textContent).toContain('claude auth login')
    expect(screen.queryByTestId('solution-generation-pending')).toBeNull()
  })

  it('the banner retry regenerates', async () => {
    state.solutions.latestRun = {
      id: 'run1',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_AI_SESSION_EXPIRED'
    }
    mount(request({ status: 'analyzing' }))
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(state.generate).toHaveBeenCalled())
  })

  it('names the code of a failed run that is not an AI setup problem', () => {
    state.solutions.latestRun = {
      id: 'run1',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_SOLUTION_INVALID_OUTPUT'
    }
    mount(request({ status: 'analyzing' }))
    expect(screen.queryByTestId('request-ai-setup-banner')).toBeNull()
    expect(screen.getByTestId('solution-generation-failed')).toBeInTheDocument()
    expect(screen.getByTestId('solution-generation-error-code').textContent).toContain(
      'REQUEST_SOLUTION_INVALID_OUTPUT'
    )
  })

  it('keeps showing the earlier analysis and puts the failure of a later regenerate above it', () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.solutions.latestRun = {
      id: 'run2',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_AI_QUOTA_EXHAUSTED'
    }
    mount(request({ type: 'bug' }))
    expect(screen.getByTestId('request-ai-setup-banner')).toHaveAttribute(
      'data-ai-code',
      'REQUEST_AI_QUOTA_EXHAUSTED'
    )
    expect(screen.getByTestId('request-tab-analysis')).toBeInTheDocument()
  })

  it('shows nothing extra when the AI works (succeeded or running run)', () => {
    state.solutions.solutions = [sol()]
    state.solutions.latestRun = { id: 'run1', kind: 'solution', status: 'succeeded' }
    mount()
    expect(screen.queryByTestId('request-ai-setup-banner')).toBeNull()
    expect(screen.queryByTestId('solution-generation-failed')).toBeNull()
  })

  it('re-reads the list while analysing, because a failed run raises no event', () => {
    vi.useFakeTimers()
    mount(request({ status: 'analyzing' }))
    expect(state.refetchS).not.toHaveBeenCalled()
    vi.advanceTimersByTime(5100)
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(5000)
    expect(state.refetchS).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  })

  it('does not poll once a run failed or a solution exists', () => {
    vi.useFakeTimers()
    state.solutions.latestRun = {
      id: 'run1',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_AI_NOT_CONFIGURED'
    }
    const first = mount(request({ status: 'analyzing' }))
    vi.advanceTimersByTime(11_000)
    first.unmount()
    state.solutions.latestRun = null
    state.solutions.solutions = [sol()]
    mount(request({ status: 'analyzing' }))
    vi.advanceTimersByTime(11_000)
    expect(state.refetchS).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('locks Regenerate while the AI class is cooling down', () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [{ ...pending, status: 'expired' }]
    mount(request({ type: 'bug' }))
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeEnabled()
    cleanup()
    startRequestCooldown('ai', 4000)
    mount(request({ type: 'bug' }))
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
  })
})

describe('Generate solution (TASK-FE-E2E-009)', () => {
  beforeEach(() => resetRequestCooldowns())

  it('offers Generate solution when analyzing with no solution and no running run', () => {
    mount(request({ status: 'analyzing' }))
    expect(screen.getByTestId('solution-start-prompt')).toBeInTheDocument()
    expect(screen.getByTestId('solution-generate')).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Generate solution' })).toBeInTheDocument()
    expect(screen.queryByTestId('solution-generation-pending')).toBeNull()
  })

  it.each([
    [
      'a solution exists',
      () => {
        state.solutions.solutions = [sol()]
      },
      request({ status: 'analyzing' })
    ],
    ['the request is not analyzing', () => undefined, request({ status: 'awaiting_information' })],
    [
      'a run is queued or running',
      () => {
        state.solutions.latestRun = { id: 'r', kind: 'solution', status: 'running' }
      },
      request({ status: 'analyzing' })
    ]
  ] as const)('does not offer it when %s', (_name, arrange, req) => {
    arrange()
    mount(req)
    expect(screen.queryByTestId('solution-generate')).toBeNull()
  })

  it('starts one run through the idle-checked call, then shows the analyzing state', async () => {
    const onChanged = vi.fn()
    mount(request({ status: 'analyzing' }), onChanged)
    const btn = screen.getByTestId('solution-generate')
    fireEvent.click(btn)
    fireEvent.click(btn)
    await waitFor(() =>
      expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
    )
    expect(state.generateIdle).toHaveBeenCalledTimes(1)
    expect(state.generate).not.toHaveBeenCalled()
    expect(state.refetchS).toHaveBeenCalled()
    expect(onChanged).toHaveBeenCalled()
    expect(screen.queryByTestId('solution-generate')).toBeNull()
  })

  it('treats a skipped call (a run already started elsewhere) as started', async () => {
    state.generateIdle.mockResolvedValue({ ok: true, value: { skipped: true } })
    mount(request({ status: 'analyzing' }))
    fireEvent.click(screen.getByTestId('solution-generate'))
    await waitFor(() =>
      expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
    )
  })

  it('shows the AI setup banner when the start fails for lack of AI, and retry calls it again', async () => {
    useAppStore.setState({ currentUser: { id: 'u1', role: 'member' } } as never)
    state.generateIdle.mockResolvedValueOnce({
      ok: false,
      error: {
        kind: 'no_ai',
        code: 'runtime_error',
        message: 'REQUEST_AI_NOT_CONFIGURED: no key',
        backendCode: 'REQUEST_AI_NOT_CONFIGURED'
      }
    })
    mount(request({ status: 'analyzing' }))
    fireEvent.click(screen.getByTestId('solution-generate'))
    expect(await screen.findByTestId('request-ai-setup-banner')).toHaveAttribute(
      'data-ai-code',
      'REQUEST_AI_NOT_CONFIGURED'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(state.generateIdle).toHaveBeenCalledTimes(2))
  })

  it('shows a coded error for a request that cannot start analysis', async () => {
    state.generateIdle.mockResolvedValueOnce({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'runtime_error',
        message: 'REQUEST_SOLUTION_WRONG_STATE: nope',
        backendCode: 'REQUEST_SOLUTION_WRONG_STATE'
      }
    })
    mount(request({ status: 'analyzing' }))
    fireEvent.click(screen.getByTestId('solution-generate'))
    expect(await screen.findByTestId('solution-generation-failed')).toBeInTheDocument()
    expect(screen.getByTestId('solution-generation-error-code').textContent).toContain(
      'REQUEST_SOLUTION_WRONG_STATE'
    )
  })

  it('locks the button while the AI class is cooling down', () => {
    startRequestCooldown('ai', 4000)
    mount(request({ status: 'analyzing' }))
    expect(screen.getByTestId('solution-generate')).toBeDisabled()
  })
})

describe('approval bar after the Solution is listed (TASK-FE-E2E-009)', () => {
  beforeEach(() => resetRequestCooldowns())
  afterEach(() => vi.useRealTimers())

  it('re-reads approvals when the request status changes after mount', () => {
    state.solutions.solutions = [sol()]
    const view = render(
      <RequestAnalysisTab request={request({ status: 'analyzing' })} onChanged={vi.fn()} />
    )
    expect(state.refetchA).not.toHaveBeenCalled()
    view.rerender(
      <RequestAnalysisTab
        request={request({ status: 'awaiting_analysis_approval' })}
        onChanged={vi.fn()}
      />
    )
    expect(state.refetchA).toHaveBeenCalledTimes(1)
  })

  it('polls approvals while awaiting analysis approval with no pending approval, then stops when it arrives', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol()]
    const view = render(<RequestAnalysisTab request={request()} onChanged={vi.fn()} />)
    expect(screen.queryByTestId('solution-approve-option')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100)
    })
    expect(state.refetchA).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(state.refetchA).toHaveBeenCalledTimes(2)
    state.approvals.approvals = [pending]
    view.rerender(<RequestAnalysisTab request={request()} onChanged={vi.fn()} />)
    expect(screen.getByTestId('solution-approve-option')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(state.refetchA).toHaveBeenCalledTimes(2)
  })

  it('gives up after a bounded number of tries', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol()]
    mount()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000 * 40)
    })
    expect(state.refetchA).toHaveBeenCalledTimes(20)
  })

  it('does not poll once the approval is expired or the analysis stage is over', async () => {
    vi.useFakeTimers()
    state.solutions.solutions = [sol()]
    state.approvals.approvals = [{ ...pending, status: 'expired' }]
    const first = mount()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    first.unmount()
    state.approvals.approvals = []
    mount(request({ status: 'planning' }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(state.refetchA).not.toHaveBeenCalled()
  })
  describe('open questions', () => {
    const questions = [
      { id: 'Q-1', text: 'Which database holds the audit trail?', blocking: true },
      { id: 'Q-2', text: 'Should we keep the old endpoint?', blocking: false }
    ]
    const blockingError = {
      kind: 'blocking_questions',
      code: 'runtime_error',
      message: 'REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1',
      backendCode: 'REQUEST_SOLUTION_BLOCKING_QUESTIONS'
    }

    it('lists blocking and non-blocking questions with the approval notice', () => {
      state.solutions.solutions = [sol({ openQuestions: questions })]
      state.approvals.approvals = [pending]
      mount()
      expect(screen.getByTestId('solution-open-questions')).toBeInTheDocument()
      const blocking = screen.getByTestId('solution-blocking-question')
      expect(blocking).toHaveAttribute('data-question-id', 'Q-1')
      expect(blocking).toHaveTextContent('Which database holds the audit trail?')
      expect(blocking).toHaveTextContent('Blocking')
      const other = screen.getByTestId('solution-open-question')
      expect(other).toHaveAttribute('data-question-id', 'Q-2')
      expect(other).toHaveTextContent('Not blocking')
      expect(screen.getByTestId('solution-blocking-notice')).toHaveAttribute(
        'data-refused',
        'false'
      )
    })

    it('shows no notice when no question blocks, and nothing when there are none', () => {
      state.solutions.solutions = [sol({ openQuestions: [questions[1]] })]
      state.approvals.approvals = [pending]
      const { unmount } = mount()
      expect(screen.getByTestId('solution-open-questions')).toBeInTheDocument()
      expect(screen.queryByTestId('solution-blocking-notice')).toBeNull()
      unmount()
      state.solutions.solutions = [sol()]
      mount()
      expect(screen.queryByTestId('solution-open-questions')).toBeNull()
    })

    it('keeps the list but drops the notice once the solution is decided', () => {
      state.solutions.solutions = [
        sol({ status: 'chosen', chosenOptionId: 'o1', openQuestions: questions })
      ]
      mount(request({ status: 'planning' }))
      expect(screen.getByTestId('solution-blocking-question')).toBeInTheDocument()
      expect(screen.queryByTestId('solution-blocking-notice')).toBeNull()
    })

    it('offers "Ask back" on blocking questions only and opens the dialog', () => {
      state.solutions.solutions = [sol({ openQuestions: questions })]
      state.approvals.approvals = [pending]
      mount()
      const buttons = screen.getAllByTestId('solution-ask-back')
      expect(buttons).toHaveLength(1)
      expect(buttons[0]).toHaveAttribute('data-question-id', 'Q-1')
      expect(screen.queryByTestId('solution-ask-back-dialog')).toBeNull()
      fireEvent.click(buttons[0])
      expect(screen.getByTestId('solution-ask-back-dialog')).toBeInTheDocument()
      expect(screen.getByTestId('solution-ask-back-submit')).toBeInTheDocument()
    })

    it('hides "Ask back" once the solution is decided or an older version', () => {
      state.solutions.solutions = [
        sol({ status: 'chosen', chosenOptionId: 'o1', openQuestions: questions })
      ]
      const { unmount } = mount(request({ status: 'planning' }))
      expect(screen.queryByTestId('solution-ask-back')).toBeNull()
      unmount()
      state.solutions.solutions = [
        sol({ id: 's0', status: 'superseded', version: 1, openQuestions: questions }),
        sol({ id: 's1', version: 2, openQuestions: [] })
      ]
      mount()
      fireEvent.click(screen.getByRole('button', { name: /Superseded/ }))
      expect(screen.queryByTestId('solution-ask-back')).toBeNull()
    })

    it('explains a refused approval instead of saying it was already decided', async () => {
      state.solutions.solutions = [sol({ openQuestions: questions })]
      state.approvals.approvals = [pending]
      state.approve.mockResolvedValue({ ok: false, error: blockingError })
      mount()
      fireEvent.click(screen.getByTestId('solution-option-o1'))
      fireEvent.click(screen.getByTestId('solution-approve-option'))
      await waitFor(() =>
        expect(screen.getByTestId('solution-blocking-notice')).toHaveAttribute(
          'data-refused',
          'true'
        )
      )
      expect(screen.getByTestId('solution-blocking-notice')).toHaveTextContent('Q-1')
      expect(screen.getByTestId('solution-blocking-question')).toHaveAttribute(
        'data-refused',
        'true'
      )
      expect(screen.getByTestId('solution-decision-bar')).toHaveTextContent(
        'blocking open questions'
      )
      expect(screen.queryByText('This was already decided.')).toBeNull()
    })

    it('shows the refused ids even when the solution carried no readable questions', async () => {
      state.solutions.solutions = [sol()]
      state.approvals.approvals = [pending]
      state.approve.mockResolvedValue({ ok: false, error: blockingError })
      mount()
      fireEvent.click(screen.getByTestId('solution-option-o1'))
      fireEvent.click(screen.getByTestId('solution-approve-option'))
      expect(await screen.findByTestId('solution-blocking-notice')).toHaveTextContent('Q-1')
      expect(screen.queryByTestId('solution-blocking-question')).toBeNull()
      expect(screen.queryByText('This was already decided.')).toBeNull()
    })
  })
})
