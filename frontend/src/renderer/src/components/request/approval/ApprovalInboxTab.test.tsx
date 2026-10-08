// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { makeApproval, makeRequest } from './approval-fixtures'

const { inboxMock, summariesMock, confirmMock, openMock, toastFn } = vi.hoisted(() => ({
  inboxMock: vi.fn(),
  summariesMock: vi.fn(),
  confirmMock: vi.fn(),
  openMock: vi.fn(),
  toastFn: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() })
}))

vi.mock('../../../hooks/useApprovalInbox', () => ({ useApprovalInbox: (...a: unknown[]) => inboxMock(...a) }))
vi.mock('../../../hooks/useRequestSummaries', () => ({ useRequestSummaries: (...a: unknown[]) => summariesMock(...a) }))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirmMock }))
vi.mock('../request-page-navigation', () => ({ openRequestPage: (...a: unknown[]) => openMock(...a) }))
vi.mock('sonner', () => ({ toast: toastFn }))

import { useAppStore } from '@/store'
import { ApprovalInboxTab } from './ApprovalInboxTab'

const base = {
  rows: [], isLoading: false, isLoadingMore: false, error: null, hasMore: false, supported: true,
  loadMore: vi.fn(), refetch: vi.fn(), approve: vi.fn(), reject: vi.fn()
}
const summaries = { byId: { 'req-1': makeRequest({ id: 'req-1' }) }, isLoading: false, failedIds: new Set(), notFoundIds: new Set() }

beforeEach(() => {
  for (const m of [inboxMock, summariesMock, confirmMock, openMock, toastFn, toastFn.success, toastFn.error]) {m.mockReset()}
  base.approve.mockReset()
  base.reject.mockReset()
  summariesMock.mockReturnValue(summaries)
  inboxMock.mockReturnValue(base)
  useAppStore.setState({ requestPage: { section: 'approvals', requestId: null, backlogView: 'requests', listFilters: {} } })
})
afterEach(cleanup)

const rowsOf = (...kinds: string[]) => kinds.map((k, i) => makeApproval({ id: `a${i}`, rawSubjectType: k, subjectType: 'unknown' }))

describe('ApprovalInboxTab', () => {
  it('shows the empty state, and the filtered variant after choosing a filter', () => {
    render(<ApprovalInboxTab />)
    expect(screen.getByTestId('approval-empty')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('radio', { name: 'Plan' }))
    expect(screen.getByTestId('approval-empty-filtered')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByTestId('approval-empty')).toBeInTheDocument()
  })

  it('shows a skeleton while loading with no rows', () => {
    inboxMock.mockReturnValue({ ...base, isLoading: true })
    render(<ApprovalInboxTab />)
    expect(screen.getByTestId('approval-skeleton')).toBeInTheDocument()
  })

  it('dims the stale list under a network banner, and shows only the message when forbidden', () => {
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('phase'), error: { kind: 'network', code: 'x', message: 'x' } })
    const { unmount } = render(<ApprovalInboxTab />)
    expect(screen.getByTestId('approval-error-network')).toBeInTheDocument()
    expect(screen.getByTestId('approval-list').className).toContain('opacity-60')
    unmount()
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('phase'), error: { kind: 'forbidden', code: 'x', message: 'x' } })
    render(<ApprovalInboxTab />)
    expect(screen.getByTestId('approval-error-forbidden')).toBeInTheDocument()
    expect(screen.queryByTestId('approval-list')).toBeNull()
  })

  it.each([
    ['request_type', 'type_confirmation'], ['solution', 'analysis'], ['findings', 'analysis'], ['answer', 'analysis'],
    ['plan', 'plan'], ['task_list', 'plan'], ['phase', 'plan'], ['pre_deploy', 'plan']
  ])('Open on %s deep-links with focus %s', (kind, focus) => {
    inboxMock.mockReturnValue({ ...base, rows: rowsOf(kind) })
    render(<ApprovalInboxTab />)
    fireEvent.click(screen.getByRole('button', { name: 'Open' }))
    expect(openMock).toHaveBeenCalledWith({ section: 'requests', requestId: 'req-1', focus })
  })

  it('quick approve asks for confirmation first, then approves; declining does nothing', async () => {
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('phase') })
    base.approve.mockResolvedValue({ outcome: 'ok' })
    confirmMock.mockResolvedValueOnce(false)
    render(<ApprovalInboxTab />)
    fireEvent.click(screen.getByRole('button', { name: /Quick approve/ }))
    await waitFor(() => expect(confirmMock).toHaveBeenCalledTimes(1))
    expect(base.approve).not.toHaveBeenCalled()
    confirmMock.mockResolvedValueOnce(true)
    fireEvent.click(screen.getByRole('button', { name: /Quick approve/ }))
    await waitFor(() => expect(base.approve).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(toastFn.success).toHaveBeenCalled())
  })

  it('closed outcome is a neutral toast, changed offers an Open action, forbidden is an error', async () => {
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('phase') })
    confirmMock.mockResolvedValue(true)
    render(<ApprovalInboxTab />)
    const click = async () => {
      fireEvent.click(screen.getByRole('button', { name: /Quick approve/ }))
      await waitFor(() => expect(base.approve).toHaveBeenCalled())
      await act(async () => {})
    }
    base.approve.mockResolvedValueOnce({ outcome: 'closed' })
    await click()
    expect(toastFn).toHaveBeenCalledTimes(1)
    expect(toastFn.error).not.toHaveBeenCalled()
    base.approve.mockReset().mockResolvedValueOnce({ outcome: 'changed' })
    await click()
    const opts = toastFn.mock.calls[1][1] as { action: { onClick: () => void } }
    opts.action.onClick()
    expect(openMock).toHaveBeenCalledWith(expect.objectContaining({ requestId: 'req-1' }))
    base.approve.mockReset().mockResolvedValueOnce({ outcome: 'forbidden' })
    await click()
    expect(toastFn.error).toHaveBeenCalledTimes(1)
  })

  it('reject needs 10 characters, sends the trimmed comment, and closes on success', async () => {
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('solution') })
    base.reject.mockResolvedValue({ outcome: 'ok' })
    render(<ApprovalInboxTab />)
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    const field = await screen.findByPlaceholderText('Why is this being rejected?')
    fireEvent.change(field, { target: { value: '123456789' } })
    const submit = screen.getAllByRole('button').find((b) => b.closest('[data-testid="reject-reason-dialog"]') && /reject/i.test(b.textContent ?? '') && !(b as HTMLButtonElement).disabled)
    if (submit) {fireEvent.click(submit)}
    expect(base.reject).not.toHaveBeenCalled()
    fireEvent.change(field, { target: { value: '  long enough reason  ' } })
    fireEvent.keyDown(field, { key: 'Enter', ctrlKey: true, metaKey: true })
    fireEvent.keyDown(field, { key: 'Enter', ctrlKey: !navigator.userAgent.includes('Mac'), metaKey: navigator.userAgent.includes('Mac') })
    await waitFor(() => expect(base.reject).toHaveBeenCalledWith(expect.objectContaining({ id: 'a0' }), 'long enough reason'))
    await waitFor(() => expect(screen.queryByTestId('reject-reason-dialog')).toBeNull())
  })

  it('hides rows whose request no longer exists and tells the user when opened', () => {
    summariesMock.mockReturnValue({ ...summaries, notFoundIds: new Set(['req-1']) })
    inboxMock.mockReturnValue({ ...base, rows: rowsOf('phase') })
    render(<ApprovalInboxTab />)
    expect(screen.queryByTestId('approval-list')).toBeNull()
  })

  it('renders nothing when the runtime does not support the flow', () => {
    inboxMock.mockReturnValue({ ...base, supported: false })
    const { container } = render(<ApprovalInboxTab />)
    expect(container).toBeEmptyDOMElement()
  })
})
