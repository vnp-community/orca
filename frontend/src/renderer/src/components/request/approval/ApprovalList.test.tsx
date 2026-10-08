// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApprovalList } from './ApprovalList'
import { groupByRequest } from './approval-inbox-rules'
import { makeApproval, makeRequest } from './approval-fixtures'

afterEach(cleanup)

function setup(extra = {}) {
  const rows = [
    makeApproval({ id: 'a1', requestId: 'r1' }),
    makeApproval({ id: 'a2', requestId: 'r2' }),
    makeApproval({ id: 'a3', requestId: 'r1' })
  ]
  const props = {
    groups: groupByRequest(rows),
    requestsById: { r1: makeRequest({ id: 'r1', number: 1, title: 'First' }), r2: makeRequest({ id: 'r2', number: 2, title: 'Second' }) },
    now: Date.parse('2026-10-07T12:00:00Z'), locale: 'en', busyIds: new Set<string>(),
    hasMore: false, isLoadingMore: false,
    onOpen: vi.fn(), onQuickApprove: vi.fn(), onReject: vi.fn(), onLoadMore: vi.fn(), ...extra
  }
  render(<ApprovalList {...props} />)
  return props
}

describe('ApprovalList', () => {
  it('groups rows under their Request heading with counts', () => {
    setup()
    expect(screen.getAllByText('#1 First').length).toBeGreaterThan(0)
    expect(screen.getAllByRole('option')).toHaveLength(3)
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('walks the flat row list with j/k and opens with Enter', () => {
    const props = setup()
    const list = screen.getByTestId('approval-list')
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'k' })
    fireEvent.keyDown(list, { key: 'Enter' })
    expect(props.onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'a3' }))
  })

  it('ignores Enter on a row button (it activates that button instead)', () => {
    const props = setup()
    fireEvent.keyDown(screen.getByTestId('approval-list'), { key: 'j' })
    fireEvent.keyDown(screen.getAllByRole('button', { name: 'Reject' })[0], { key: 'Enter' })
    expect(props.onOpen).not.toHaveBeenCalled()
  })

  it('offers load more only when there is another page', () => {
    cleanup()
    const props = setup({ hasMore: true })
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(props.onLoadMore).toHaveBeenCalled()
  })
})
