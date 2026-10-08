// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../hooks/useRequestFlowSupport', () => ({ useRequestFlowSupport: () => undefined }))
vi.mock('./RequestsTab', () => ({ RequestsTab: () => <div data-testid="request-tab-requests" /> }))
vi.mock('./approval/ApprovalInboxTab', () => ({ ApprovalInboxTab: () => <div data-testid="request-tab-approvals" /> }))
vi.mock('./backlog/BacklogTab', () => ({ BacklogTab: () => <div data-testid="request-tab-backlog" /> }))

import { useAppStore } from '@/store'
import RequestPage from './RequestPage'

beforeEach(() =>
  useAppStore.setState({
    requestFlowSupport: 'supported',
    activeView: 'requests',
    requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: {} }
  })
)
afterEach(cleanup)

describe('RequestPage', () => {
  it('shows a skeleton while support is unknown and the notice when unsupported', () => {
    useAppStore.setState({ requestFlowSupport: 'unknown' })
    const { container, unmount } = render(<RequestPage />)
    expect(container.querySelector('.animate-pulse')).not.toBeNull()
    unmount()
    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    render(<RequestPage />)
    expect(screen.getByText('Request management not supported')).toBeInTheDocument()
  })

  it('switches tabs through the store', () => {
    render(<RequestPage />)
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Approvals' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Approvals' }))
    expect(useAppStore.getState().requestPage.section).toBe('approvals')
  })

  it('Escape closes the page, but not while typing in an input', () => {
    render(
      <>
        <RequestPage />
        <input data-testid="field" />
      </>
    )
    screen.getByTestId('field').focus()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(useAppStore.getState().activeView).toBe('requests')
    screen.getByTestId('field').blur()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(useAppStore.getState().activeView).toBe('terminal')
  })

  it('offers Create request in the header when supported', () => {
    render(<RequestPage />)
    fireEvent.click(screen.getByRole('button', { name: 'Create request' }))
    expect(screen.getByLabelText('Title')).toBeInTheDocument()
  })
})
