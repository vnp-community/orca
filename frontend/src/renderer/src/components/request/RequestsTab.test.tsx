// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useRequestsMock = vi.fn()
const narrowMock = vi.fn(() => false)
vi.mock('../../hooks/useRequests', () => ({ useRequests: (...a: unknown[]) => useRequestsMock(...a) }))
vi.mock('../../hooks/useRequestNarrowLayout', () => ({ useRequestNarrowLayout: () => narrowMock() }))
vi.mock('./RequestDetailPane', () => ({
  RequestDetailPane: ({ requestId }: { requestId: string }) => <div data-testid="detail">{requestId}</div>
}))

import { useAppStore } from '@/store'
import { RequestsTab } from './RequestsTab'

class RO {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

const req = (id: string, n: number) => ({
  id, number: n, title: `Title ${id}`, type: 'bug', status: 'submitted', projectId: 'p',
  createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z'
})
const base = { requests: [], isLoading: false, hasLoaded: true, error: null, nextPageToken: null, nextPage: vi.fn(), refetch: vi.fn(), supported: true }

beforeEach(() => {
  ;(globalThis as { ResizeObserver?: unknown }).ResizeObserver = RO
  narrowMock.mockReturnValue(false)
  useRequestsMock.mockReset()
  useAppStore.setState({
    requestFlowSupport: 'supported',
    requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: {} }
  })
})
afterEach(cleanup)

describe('RequestsTab', () => {
  it('shows the skeleton before the first load settles (no empty flash)', () => {
    useRequestsMock.mockReturnValue({ ...base, hasLoaded: false })
    render(<RequestsTab />)
    expect(screen.getByTestId('request-list-skeleton')).toBeInTheDocument()
    expect(screen.queryByTestId('request-list-empty')).toBeNull()
  })

  it('shows the empty state, with a create button when unfiltered', () => {
    const onCreate = vi.fn()
    useRequestsMock.mockReturnValue(base)
    render(<RequestsTab onCreate={onCreate} />)
    fireEvent.click(screen.getByText('Create request'))
    expect(onCreate).toHaveBeenCalled()
  })

  it('shows the filtered empty state and clears filters', () => {
    useAppStore.setState({ requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: { type: ['bug'] } } })
    useRequestsMock.mockReturnValue(base)
    render(<RequestsTab />)
    expect(screen.getByText('No requests match these filters.')).toBeInTheDocument()
    fireEvent.click(screen.getAllByText('Clear filters')[0])
    expect(useAppStore.getState().requestPage.listFilters).toEqual({})
  })

  it('shows a retryable network error and a plain forbidden error', () => {
    const refetch = vi.fn()
    useRequestsMock.mockReturnValue({ ...base, error: 'network', refetch })
    const { unmount } = render(<RequestsTab />)
    fireEvent.click(screen.getByText('Retry'))
    expect(refetch).toHaveBeenCalled()
    unmount()
    useRequestsMock.mockReturnValue({ ...base, error: 'forbidden' })
    render(<RequestsTab />)
    expect(screen.getByTestId('request-list-error')).toBeInTheDocument()
    expect(screen.queryByText('Retry')).toBeNull()
  })

  it('selecting a row stores the request id and shows the detail', () => {
    useRequestsMock.mockReturnValue({ ...base, requests: [req('a', 1), req('b', 2)] })
    render(<RequestsTab />)
    fireEvent.click(screen.getByText('Title b'))
    expect(useAppStore.getState().requestPage.requestId).toBe('b')
    cleanup()
    render(<RequestsTab />)
    expect(screen.getByTestId('detail')).toHaveTextContent('b')
  })

  it('quick chips write the status filter', () => {
    useRequestsMock.mockReturnValue(base)
    render(<RequestsTab />)
    fireEvent.click(screen.getByText('Running'))
    expect(useAppStore.getState().requestPage.listFilters.status).toEqual(['analyzing', 'planning', 'executing'])
    fireEvent.click(screen.getByText('Needs my confirmation'))
    expect(useAppStore.getState().requestPage.listFilters.status).toEqual(['awaiting_type_confirmation'])
  })

  it('load more appends via nextPage', () => {
    const nextPage = vi.fn()
    useRequestsMock.mockReturnValue({ ...base, requests: [req('a', 1)], nextPageToken: 't', nextPage })
    render(<RequestsTab />)
    fireEvent.click(screen.getByText('Load more'))
    expect(nextPage).toHaveBeenCalled()
  })

  it('narrow layout shows the detail instead of the list', () => {
    narrowMock.mockReturnValue(true)
    useRequestsMock.mockReturnValue({ ...base, requests: [req('a', 1)] })
    useAppStore.setState({ requestPage: { section: 'requests', requestId: 'a', backlogView: 'requests', listFilters: {} } })
    render(<RequestsTab />)
    expect(screen.getByTestId('detail')).toBeInTheDocument()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('j/k move the selection inside the list', () => {
    useRequestsMock.mockReturnValue({ ...base, requests: [req('a', 1), req('b', 2)] })
    useAppStore.setState({ requestPage: { section: 'requests', requestId: 'a', backlogView: 'requests', listFilters: {} } })
    render(<RequestsTab />)
    fireEvent.keyDown(screen.getByRole('listbox'), { key: 'j' })
    expect(useAppStore.getState().requestPage.requestId).toBe('b')
  })
})
