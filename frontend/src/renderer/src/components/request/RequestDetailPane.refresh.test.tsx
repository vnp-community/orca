// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useRequestMock = vi.fn()
vi.mock('../../hooks/useRequest', () => ({ useRequest: (...a: unknown[]) => useRequestMock(...a) }))
vi.mock('../../hooks/useRequestActions', () => ({ useRequestActions: () => ({}) }))
vi.mock('../../runtime/request-rpc-client', () => ({
  callRequestRpc: vi.fn(async () => ({ ok: false, error: { kind: 'not_found' } }))
}))
// The tab is replaced by a probe so the test sees exactly what the pane hands it.
vi.mock('./RequestOverviewTab', () => ({
  RequestOverviewTab: ({ onChanged }: { onChanged?: () => void }) => (
    <button type="button" onClick={onChanged}>
      overview-changed
    </button>
  )
}))

import { RequestDetailPane } from './RequestDetailPane'

const refetch = vi.fn()
const request = {
  id: 'r1',
  number: 7,
  title: 'Fix login',
  body: 'Body',
  type: 'change_request',
  status: 'analyzing',
  projectId: 'p',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z'
}

beforeEach(() => {
  useRequestMock.mockReturnValue({
    request,
    history: [],
    links: [],
    linksSupported: true,
    isLoading: false,
    error: null,
    refetch
  })
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('RequestDetailPane refresh of the starting branch', () => {
  it('refetches the Request when the Overview says the starting branch changed', () => {
    render(<RequestDetailPane requestId="r1" />)
    fireEvent.click(screen.getByText('overview-changed'))
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('refetches when the window is focused again (no gateway event announces a branch change)', () => {
    render(<RequestDetailPane requestId="r1" />)
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(refetch).toHaveBeenCalledTimes(1)
  })
})
