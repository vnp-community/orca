// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Approval, OrcaRequest, Solution } from '../../../../../shared/request-types'

const state = vi.hoisted(() => ({
  solutions: [] as unknown[],
  approvals: [] as unknown[],
  choose: vi.fn(),
  generate: vi.fn(),
  approve: vi.fn(),
  reject: vi.fn(),
  refetchS: vi.fn(),
  refetchA: vi.fn()
}))

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('../../../hooks/useSolutions', () => ({
  useSolutions: () => ({
    solutions: state.solutions,
    isLoading: false,
    error: null,
    latestRun: null,
    generate: state.generate,
    generateWhenIdle: vi.fn(),
    choose: state.choose,
    refetch: state.refetchS
  })
}))
vi.mock('../../../hooks/useApprovals', () => ({
  useApprovals: () => ({
    approvals: state.approvals,
    approve: state.approve,
    reject: state.reject,
    refetch: state.refetchA,
    pendingCount: 0,
    isLoading: false,
    error: null
  })
}))
vi.mock('../../../hooks/useRequestActions', () => ({
  useRequestActions: () => ({ generatePlan: vi.fn() })
}))

import { resetRequestCooldowns } from '../../../runtime/request-rate-limit'
import { RequestAnalysisTab } from './RequestAnalysisTab'

afterEach(cleanup)
beforeEach(() => {
  resetRequestCooldowns()
  state.solutions = []
  state.approvals = []
  for (const f of [
    state.choose,
    state.generate,
    state.approve,
    state.reject,
    state.refetchS,
    state.refetchA
  ]) {
    f.mockReset()
    f.mockResolvedValue({ ok: true, value: {} })
  }
})

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1',
  projectId: 'p',
  number: 1,
  title: 't',
  type: 'change_request',
  status: 'awaiting_analysis_approval',
  createdAt: '',
  updatedAt: 'u1',
  version: 1,
  ...over
})
const sol = (over: Partial<Solution> = {}): Solution => ({
  id: 's1',
  requestId: 'r1',
  kind: 'solution',
  status: 'ready',
  seq: 1,
  options: [
    { id: 'o1', title: 'Alpha', summary: 'a' },
    { id: 'o2', title: 'Beta', summary: 'b' }
  ],
  ...over
})
const pending = (subjectId = 's1'): Approval =>
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

const mount = (r = request()) => render(<RequestAnalysisTab request={r} onChanged={vi.fn()} />)
const type = (text: string): void => {
  fireEvent.change(screen.getByTestId('analysis-revision-input'), { target: { value: text } })
}

describe('revision panel in SolutionPanel', () => {
  it('shows at the Analysis gate on the current version, next to the decision bar', () => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    mount()
    expect(screen.getByTestId('analysis-revision-panel')).toBeInTheDocument()
    expect(screen.getByTestId('solution-decision-bar')).toBeInTheDocument()
  })

  it.each([
    ['planning', 'a Request past the Analysis gate'],
    ['analyzing', 'a Request that is analysing with no revision under way'],
    ['executing', 'an executing Request']
  ] as const)('is hidden for status %s (%s)', (status, _why) => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    mount(request({ status }))
    expect(screen.queryByTestId('analysis-revision-panel')).toBeNull()
  })

  it('is hidden for a type that has no Analysis gate', () => {
    state.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals = [pending()]
    mount(request({ type: 'hotfix' }))
    expect(screen.queryByTestId('analysis-revision-panel')).toBeNull()
  })

  it('is hidden when the version is decided (no decision bar)', () => {
    state.solutions = [sol({ status: 'chosen', chosenOptionId: 'o1' })]
    mount()
    expect(screen.queryByTestId('analysis-revision-panel')).toBeNull()
  })

  it('sends the typed text as feedback to generate, never through reject, and clears the box', async () => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    mount()
    type('Add a rollback plan')
    fireEvent.click(screen.getByTestId('analysis-revision-submit'))
    await waitFor(() => expect(state.generate).toHaveBeenCalledTimes(1))
    expect(state.generate).toHaveBeenCalledWith({
      feedback: 'Add a rollback plan',
      idempotencyKey: expect.any(String)
    })
    expect(state.reject).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.getByTestId('analysis-revision-input')).toHaveValue(''))
  })

  it('keeps the typed text and explains an error under the field', async () => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    state.generate.mockResolvedValue({
      ok: false,
      error: { kind: 'forbidden', code: 'forbidden', message: 'REQUEST_SOLUTION_FORBIDDEN: no' }
    })
    mount()
    type('Cover rollback')
    fireEvent.click(screen.getByTestId('analysis-revision-submit'))
    expect(await screen.findByTestId('analysis-revision-error')).toHaveTextContent(
      'Only the approvers'
    )
    expect(screen.getByTestId('analysis-revision-input')).toHaveValue('Cover rollback')
  })

  it('locks every decision while the new version is being written, then unlocks', async () => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    const { rerender } = mount()
    fireEvent.click(screen.getByTestId('solution-option-o1'))
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeEnabled()
    type('Compare on cost')
    fireEvent.click(screen.getByTestId('analysis-revision-submit'))
    await waitFor(() => expect(state.generate).toHaveBeenCalled())

    // The Request leaves the gate while the AI writes version 2: the panel stays and says so.
    state.solutions = [
      sol({ id: 's2', seq: 2, status: 'generating' }),
      sol({ status: 'superseded' })
    ]
    rerender(
      <RequestAnalysisTab
        request={request({ status: 'analyzing', updatedAt: 'u2', version: 2 })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByTestId('analysis-revision-status')).toHaveTextContent('Analyzing version 2…')
    expect(screen.getByTestId('analysis-revision-submit')).toBeDisabled()

    // The new version lands and the Request is back at the gate.
    state.solutions = [
      sol({ id: 's2', seq: 2, revisionFeedback: 'Compare on cost' }),
      sol({ status: 'superseded' })
    ]
    state.approvals = [pending('s2')]
    rerender(
      <RequestAnalysisTab request={request({ updatedAt: 'u3', version: 3 })} onChanged={vi.fn()} />
    )
    expect(screen.getByTestId('analysis-revision-status')).toBeEmptyDOMElement()
    expect(screen.getByTestId('analysis-revision-submit')).toBeEnabled()
    expect(screen.queryByTestId('solution-older-version-banner')).toBeNull()
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'ready')
  })

  it('disables Approve and Reject while busy', async () => {
    state.solutions = [sol()]
    state.approvals = [pending()]
    const { rerender } = mount()
    type('x')
    fireEvent.click(screen.getByTestId('analysis-revision-submit'))
    await waitFor(() => expect(state.generate).toHaveBeenCalled())
    rerender(
      <RequestAnalysisTab
        request={request({ status: 'analyzing', updatedAt: 'u2', version: 2 })}
        onChanged={vi.fn()}
      />
    )
    const bar = screen.getByTestId('solution-decision-bar')
    expect(within(bar).getByRole('button', { name: 'Approve this option' })).toBeDisabled()
    expect(within(bar).getByRole('button', { name: 'Reject' })).toBeDisabled()
  })

  it('disables Regenerate while busy when the approval row has not arrived yet', async () => {
    state.solutions = [sol()]
    const { rerender } = mount()
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeEnabled()
    type('x')
    fireEvent.click(screen.getByTestId('analysis-revision-submit'))
    await waitFor(() => expect(state.generate).toHaveBeenCalled())
    rerender(
      <RequestAnalysisTab
        request={request({ status: 'analyzing', updatedAt: 'u2', version: 2 })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
  })
})

describe('older versions', () => {
  const twoVersions = (): void => {
    state.solutions = [
      sol({ id: 's2', seq: 2, revisionFeedback: 'Compare on cost\n<script>alert(1)</script>' }),
      sol({ id: 's1', seq: 1, status: 'superseded', openQuestions: [] })
    ]
    state.approvals = [pending('s2')]
  }

  it('opens read-only with a banner, no decision actions and no revision panel; Back to current returns', () => {
    twoVersions()
    mount()
    expect(screen.queryByTestId('solution-older-version-banner')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    expect(screen.getByTestId('solution-older-version-banner')).toHaveTextContent(
      "You're viewing an older version."
    )
    expect(screen.queryByTestId('solution-decision-bar')).toBeNull()
    expect(screen.queryByTestId('analysis-revision-panel')).toBeNull()
    for (const name of [/Approve/, /Choose/, /Reject/, /Revise/, /Regenerate/]) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
    fireEvent.click(screen.getByTestId('solution-back-to-current'))
    expect(screen.queryByTestId('solution-older-version-banner')).toBeNull()
    expect(screen.getByTestId('solution-decision-bar')).toBeInTheDocument()
    expect(screen.getByTestId('analysis-revision-panel')).toBeInTheDocument()
  })

  it('selecting the current version in the list also leaves the older view', () => {
    twoVersions()
    mount()
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    fireEvent.click(screen.getByRole('button', { name: /Version 2/ }))
    expect(screen.queryByTestId('solution-older-version-banner')).toBeNull()
    expect(screen.getByTestId('solution-decision-bar')).toBeInTheDocument()
  })

  it('shows what was asked for a version as plain text, never as markup', () => {
    twoVersions()
    const { container } = mount()
    const block = screen.getByTestId('solution-revision-feedback')
    fireEvent.click(within(block).getByRole('button', { name: 'What was asked for this version' }))
    const text = screen.getByTestId('solution-revision-feedback-text')
    expect(text).toHaveTextContent('<script>alert(1)</script>')
    expect(text.textContent).toBe('Compare on cost\n<script>alert(1)</script>')
    expect(container.querySelector('script')).toBeNull()
    expect(text).toHaveClass('whitespace-pre-wrap')
  })

  it('has no feedback block for the first proposal', () => {
    twoVersions()
    mount()
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    expect(screen.queryByTestId('solution-revision-feedback')).toBeNull()
  })

  it('keeps an older rejected version readable but not decidable', () => {
    state.solutions = [
      sol({ id: 's2', seq: 2 }),
      sol({ id: 's1', seq: 1, status: 'rejected', rejectionReason: 'Too risky for now.' })
    ]
    state.approvals = [pending('s2')]
    mount()
    expect(screen.getByRole('button', { name: /Version 1/ })).toHaveTextContent('Rejected')
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    expect(screen.queryByTestId('solution-decision-bar')).toBeNull()
    expect(screen.getByTestId('solution-older-version-banner')).toBeInTheDocument()
  })
})
