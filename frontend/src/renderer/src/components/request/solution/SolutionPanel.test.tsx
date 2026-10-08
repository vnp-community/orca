// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Approval, OrcaRequest, Solution } from '../../../../../shared/request-types'

const state = vi.hoisted(() => ({
  solutions: { solutions: [] as unknown[], isLoading: false, error: null as string | null },
  approvals: { approvals: [] as unknown[] },
  choose: vi.fn(),
  generate: vi.fn(),
  approve: vi.fn(),
  reject: vi.fn(),
  refetchS: vi.fn(),
  refetchA: vi.fn(),
  generatePlan: vi.fn()
}))

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('../../../hooks/useSolutions', () => ({
  useSolutions: () => ({ ...state.solutions, generate: state.generate, choose: state.choose, refetch: state.refetchS })
}))
vi.mock('../../../hooks/useApprovals', () => ({
  useApprovals: () => ({ ...state.approvals, approve: state.approve, reject: state.reject, refetch: state.refetchA, pendingCount: 0, isLoading: false, error: null })
}))
vi.mock('../../../hooks/useRequestActions', () => ({ useRequestActions: () => ({ generatePlan: state.generatePlan }) }))

import { RequestAnalysisTab } from './RequestAnalysisTab'
import { emitRequestEvent } from '../../../lib/request-event-bus'

afterEach(cleanup)
beforeEach(() => {
  state.solutions = { solutions: [], isLoading: false, error: null }
  state.approvals = { approvals: [] }
  for (const f of [state.choose, state.generate, state.approve, state.reject, state.refetchS, state.refetchA, state.generatePlan]) {
    f.mockReset()
    f.mockResolvedValue({ ok: true, value: {} })
  }
})

const request = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1', projectId: 'p', number: 1, title: 't', type: 'change_request', status: 'awaiting_analysis_approval',
  createdAt: '', updatedAt: '', ...over
})
const sol = (over: Partial<Solution> = {}): Solution => ({
  id: 's1', requestId: 'r1', kind: 'solution', status: 'ready', version: 1,
  options: [{ id: 'o1', title: 'Alpha', summary: 'a' }, { id: 'o2', title: 'Beta', summary: 'b' }], ...over
})
const pending = { id: 'ap1', requestId: 'r1', subjectType: 'solution', subjectId: 's1', status: 'pending', version: 2, subjectDigest: 'dg', createdAt: '', updatedAt: '' } as Approval

const mount = (r = request(), onChanged = vi.fn()) => render(<RequestAnalysisTab request={r} onChanged={onChanged} />)

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
    state.solutions = { solutions: [], isLoading: false, error: 'network' }
    const second = mount()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(state.refetchS).toHaveBeenCalled()
    second.unmount()
    state.solutions.error = 'forbidden'
    mount()
    expect(screen.getByRole('alert')).toHaveTextContent('permission to view')
  })

  it('shows the analyzing state when no solution exists yet', () => {
    mount(request({ status: 'analyzing' }))
    expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
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
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined, content: JSON.stringify({ root_cause: 'Null deref' }) })]
    mount(request({ type: 'bug' }))
    expect(screen.getByText('Null deref')).toBeInTheDocument()
    expect(screen.getAllByText('No data').length).toBe(2)
  })

  it('shows drafting banner for generating status', () => {
    state.solutions.solutions = [sol({ status: 'generating' })]
    mount()
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'generating')
    expect(screen.getByTestId('solution-generation-pending')).toBeInTheDocument()
  })

  it('collapses superseded versions with an "Older version" label and is read-only', () => {
    state.solutions.solutions = [sol({ id: 's0', status: 'superseded', version: 1 }), sol({ id: 's1', version: 2 })]
    mount()
    expect(screen.getByRole('button', { name: /Older version/ })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Older version/ }))
    expect(screen.getByTestId('solution-status-banner')).toHaveAttribute('data-status', 'superseded')
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
    expect(state.choose.mock.invocationCallOrder[0]).toBeLessThan(state.approve.mock.invocationCallOrder[0])
    expect(state.approve.mock.calls[0][0].approval).toMatchObject({ id: 'ap1', version: 2, subjectDigest: 'fresh' })
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
  })

  it('reject dialog sends approval.reject with the comment', async () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [pending]
    mount(request({ type: 'bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    const box = screen.getByRole('textbox')
    fireEvent.change(box, { target: { value: 'too short' } })
    expect(screen.getByRole('dialog').querySelector('button[data-variant="destructive"]')).toBeDisabled()
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
    state.approve.mockResolvedValue({ ok: false, error: { kind: 'unknown', code: 'x', message: 'APPROVAL_NOT_APPROVER: no' } })
    mount(request({ type: 'bug' }))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    expect(await screen.findByText('You do not have permission to approve')).toBeInTheDocument()
  })

  it('conflict refetches and shows the updated banner', async () => {
    state.solutions.solutions = [sol({ kind: 'diagnosis', options: undefined })]
    state.approvals.approvals = [pending]
    state.approve.mockResolvedValue({ ok: false, error: { kind: 'conflict', code: 'x', message: 'APPROVAL_VERSION_CONFLICT: stale' } })
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
    emitRequestEvent({ requestId: 'r1', eventType: 'orca.request.solution.proposed', occurredAt: '' })
    expect(state.refetchS).toHaveBeenCalledTimes(1)
    emitRequestEvent({ requestId: 'other', eventType: 'orca.request.solution.proposed', occurredAt: '' })
    emitRequestEvent({ requestId: 'r1', eventType: 'orca.request.request.classified', occurredAt: '' })
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
