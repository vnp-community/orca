// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { reqHook, taskHook, execHook } = vi.hoisted(() => ({ reqHook: vi.fn(), taskHook: vi.fn(), execHook: vi.fn() }))
vi.mock('../../../hooks/useBacklog', () => ({
  useRequestBacklog: (...a: unknown[]) => reqHook(...a),
  useTaskBacklog: (...a: unknown[]) => taskHook(...a),
  useExecuteBacklog: (...a: unknown[]) => execHook(...a)
}))
vi.mock('../../../hooks/useRequestSummaries', () => ({
  useRequestSummaries: () => ({ byId: {}, isLoading: false, failedIds: new Set(), notFoundIds: new Set() })
}))
vi.mock('../../../hooks/useRequestActions', () => ({ useRequestActions: () => ({ reopen: vi.fn(), cancel: vi.fn() }) }))
vi.mock('../../task/TaskDetail', () => ({ TaskDetail: () => null }))

import { useAppStore } from '@/store'
import { BacklogTab } from './BacklogTab'
import { backlogGroup, backlogRow } from './backlog-fixtures'

function state(over: Record<string, unknown> = {}) {
  return {
    items: [], nextPageToken: null, isLoading: false, isLoadingMore: false, error: null, loadedOnce: true,
    hasMore: false, supported: true, loadMore: vi.fn(), refetch: vi.fn(),
    countLabel: () => ({ count: 0, plus: false }), ...over
  }
}

beforeEach(() => {
  reqHook.mockReturnValue(state())
  taskHook.mockReturnValue(state())
  execHook.mockReturnValue(state())
  useAppStore.setState({
    requestFlowSupport: 'supported',
    requestPage: { section: 'backlog', requestId: null, backlogView: 'requests', listFilters: {} }
  })
})
afterEach(() => { cleanup(); reqHook.mockReset(); taskHook.mockReset(); execHook.mockReset() })

const activeFlags = () => [reqHook, taskHook, execHook].map((h) => (h.mock.calls.at(-1)![1] as { active: boolean }).active)

describe('BacklogTab', () => {
  it('only activates the open view, and switching views with 1/2/3 keeps the others inactive', () => {
    render(<BacklogTab />)
    expect(activeFlags()).toEqual([true, false, false])
    fireEvent.keyDown(screen.getByTestId('request-tab-backlog'), { key: '3' })
    expect(useAppStore.getState().requestPage.backlogView).toBe('execute')
    expect(activeFlags()).toEqual([false, false, true])
  })

  it('does not switch view when typing in the search box', () => {
    render(<BacklogTab />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: '2' })
    expect(useAppStore.getState().requestPage.backlogView).toBe('requests')
  })

  it('shows per-view empty states', () => {
    render(<BacklogTab />)
    expect(screen.getByText('No Request has been returned to the backlog.')).toBeInTheDocument()
    act(() => useAppStore.getState().setRequestPageData({ backlogView: 'tasks' }))
    expect(screen.getByText('Every task already has an approved Plan.')).toBeInTheDocument()
    act(() => useAppStore.getState().setRequestPageData({ backlogView: 'execute' }))
    expect(screen.getByText('No task is waiting to run or failed.')).toBeInTheDocument()
  })

  it('shows a skeleton before the first load', () => {
    reqHook.mockReturnValue(state({ loadedOnce: false, isLoading: true }))
    render(<BacklogTab />)
    expect(screen.getByTestId('backlog-skeleton').children).toHaveLength(8)
  })

  it('an error in the task view leaves the request view intact', () => {
    reqHook.mockReturnValue(state({ items: [backlogRow()], countLabel: () => ({ count: 1, plus: false }) }))
    taskHook.mockReturnValue(state({ loadedOnce: false, error: { kind: 'network', code: 'x' } }))
    const { unmount } = render(<BacklogTab />)
    expect(screen.getByTestId('request-backlog-table')).toBeInTheDocument()
    expect(screen.queryByTestId('backlog-error-network')).toBeNull()
    unmount()
    useAppStore.getState().setRequestPageData({ backlogView: 'tasks' })
    render(<BacklogTab />)
    expect(screen.getByTestId('backlog-error-network')).toBeInTheDocument()
  })

  it('keeps stale rows dimmed under the network banner; forbidden shows only the message', () => {
    reqHook.mockReturnValue(state({ items: [backlogRow()], error: { kind: 'network', code: 'x' } }))
    const { unmount } = render(<BacklogTab />)
    expect(screen.getByTestId('backlog-error-network')).toBeInTheDocument()
    expect(screen.getByTestId('request-backlog-table').className).toContain('opacity-60')
    unmount()
    reqHook.mockReturnValue(state({ items: [backlogRow()], error: { kind: 'forbidden', code: 'x' } }))
    render(<BacklogTab />)
    expect(screen.getByTestId('backlog-error-forbidden')).toBeInTheDocument()
    expect(screen.queryByTestId('request-backlog-table')).toBeNull()
  })

  it('renders nothing when unsupported', () => {
    reqHook.mockReturnValue(state({ supported: false }))
    const { container } = render(<BacklogTab />)
    expect(container).toBeEmptyDOMElement()
  })

  it('filters on the client and offers "Clear filters" when nothing matches', async () => {
    vi.useFakeTimers()
    reqHook.mockReturnValue(state({ items: [backlogRow({ title: 'Alpha' })] }))
    render(<BacklogTab />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'zzz' } })
    act(() => { vi.advanceTimersByTime(250) })
    vi.useRealTimers()
    expect(screen.getByTestId('backlog-empty-filtered')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }))
    expect(screen.getByTestId('request-backlog-table')).toBeInTheDocument()
  })

  it('renders group tables for task and execute views', () => {
    taskHook.mockReturnValue(state({ items: [backlogGroup()] }))
    useAppStore.getState().setRequestPageData({ backlogView: 'tasks' })
    render(<BacklogTab />)
    expect(screen.getByTestId('tasks-backlog-table')).toBeInTheDocument()
  })
})
