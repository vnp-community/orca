// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApprovalRow } from './ApprovalRow'
import { canQuickApprove } from './approval-inbox-rules'
import { makeApproval, makeRequest } from './approval-fixtures'

afterEach(cleanup)
const NOW = Date.parse('2026-10-07T12:00:00Z')

function renderRow(over = {}, props = {}) {
  const handlers = { onOpen: vi.fn(), onQuickApprove: vi.fn(), onReject: vi.fn() }
  const approval = makeApproval({ id: 'a1', ...over })
  render(
    <ApprovalRow
      approval={approval}
      request={makeRequest({ id: 'req-1' })}
      now={NOW}
      locale="en"
      canQuick={canQuickApprove(approval)}
      busy={false}
      {...handlers}
      {...props}
    />
  )
  return handlers
}

describe('ApprovalRow', () => {
  const kinds: [string, boolean][] = [
    ['request_type', true], ['solution', false], ['findings', true], ['answer', true],
    ['plan', false], ['phase', true], ['task_list', true], ['pre_deploy', true]
  ]
  it.each(kinds)('%s shows quick approve = %s, always Open and Reject', (kind, quick) => {
    renderRow({ rawSubjectType: kind, subjectType: 'unknown' })
    expect(screen.queryByRole('button', { name: /Quick approve/ }) !== null).toBe(quick)
    expect(screen.getByRole('button', { name: 'Open' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()
  })

  it('emits the row for each action', () => {
    const h = renderRow()
    fireEvent.click(screen.getByRole('button', { name: 'Open' }))
    fireEvent.click(screen.getByRole('button', { name: /Quick approve/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }))
    expect(h.onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'a1' }))
    expect(h.onQuickApprove).toHaveBeenCalledTimes(1)
    expect(h.onReject).toHaveBeenCalledTimes(1)
  })

  it('flags overdue rows with a warning', () => {
    renderRow({ dueAt: '2026-10-05T12:00:00Z' })
    expect(screen.getByTestId('approval-overdue')).toBeInTheDocument()
  })

  it('does not show a due label without a deadline', () => {
    renderRow()
    expect(screen.queryByTestId('approval-overdue')).toBeNull()
    expect(screen.queryByText(/Due /)).toBeNull()
  })

  it('locks the buttons while busy', () => {
    renderRow({}, { busy: true })
    for (const b of screen.getAllByRole('button')) {expect(b).toBeDisabled()}
  })

  it('shows a short id while the request summary has not loaded, and AI / System for system actors', () => {
    renderRow({ requestId: 'abcdefgh1234', requestedBy: 'system' }, { request: undefined })
    expect(screen.getByText('#abcdefgh')).toBeInTheDocument()
    expect(screen.getByText(/AI \/ System/)).toBeInTheDocument()
    expect(screen.getByTestId('approval-row')).toHaveAttribute('aria-busy', 'true')
  })
})
