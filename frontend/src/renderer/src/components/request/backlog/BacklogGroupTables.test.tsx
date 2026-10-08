// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { openMock, summariesMock } = vi.hoisted(() => ({ openMock: vi.fn(), summariesMock: vi.fn() }))
vi.mock('../request-page-navigation', () => ({ openRequestPage: (...a: unknown[]) => openMock(...a) }))
vi.mock('../../../hooks/useRequestSummaries', () => ({ useRequestSummaries: (...a: unknown[]) => summariesMock(...a) }))
vi.mock('../../task/TaskDetail', () => ({ TaskDetail: () => <div data-testid="task-detail" /> }))

import { useAppStore } from '@/store'
import { ExecuteBacklogTable } from './ExecuteBacklogTable'
import { TaskBacklogTable } from './TaskBacklogTable'
import { backlogGroup, backlogTask } from './backlog-fixtures'

beforeEach(() => {
  openMock.mockReset()
  summariesMock.mockReturnValue({ byId: {}, isLoading: false, failedIds: new Set(), notFoundIds: new Set() })
  useAppStore.setState({ tasks: [], activeTaskId: 'previous' })
})
afterEach(cleanup)

const common = { hasMore: false, isLoadingMore: false, onLoadMore: vi.fn(), onTaskSheetClosed: vi.fn() }

describe('TaskBacklogTable', () => {
  const groups = [
    backlogGroup({
      tasks: [
        backlogTask({ taskId: 't1', title: 'Alpha', estimatedHours: 4, blockedByTaskIds: ['x1', 'x2', 'x3'] }),
        backlogTask({ taskId: 't2', title: 'Beta' })
      ]
    }),
    backlogGroup({ requestId: 'r2', planTaskId: 'pl2', planTitle: 'Loose plan', gateStatus: 'none', tasks: [backlogTask({ taskId: 't3', title: 'Gamma' })] })
  ]

  it('renders a header per group, estimate fallbacks and destructive dependency chips with +N', () => {
    useAppStore.setState({ tasks: [{ id: 'x1', title: 'Setup db' } as never] })
    render(<TaskBacklogTable groups={groups} {...common} />)
    expect(screen.getAllByTestId('backlog-group-header')).toHaveLength(2)
    expect(screen.getByText('4h')).toBeInTheDocument()
    const chips = screen.getAllByTestId('dependency-chip')
    expect(chips).toHaveLength(2)
    expect(chips[0]).toHaveTextContent('Setup db')
    expect(chips[1]).toHaveTextContent('x2')
    expect(screen.getByText('+1')).toBeInTheDocument()
    expect(chips[0].className).toContain('bg-destructive')
  })

  it('flags a plan without phases only when gate is none and phaseTaskId is empty', () => {
    render(<TaskBacklogTable groups={groups} {...common} />)
    expect(screen.getAllByText('Not split into phases yet')).toHaveLength(1)
  })

  it('opens the plan with focus=plan', () => {
    render(<TaskBacklogTable groups={groups} {...common} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Open plan' })[0])
    expect(openMock).toHaveBeenCalledWith({ section: 'requests', requestId: 'r1', focus: 'plan' })
  })

  it('Enter on the selected task opens the sheet with setActiveTask, closing restores the previous task', () => {
    const closed = vi.fn()
    render(<TaskBacklogTable groups={groups} {...common} onTaskSheetClosed={closed} />)
    const grid = screen.getByTestId('tasks-backlog-table')
    fireEvent.keyDown(grid, { key: 'j' })
    fireEvent.keyDown(grid, { key: 'Enter' })
    expect(useAppStore.getState().activeTaskId).toBe('t1')
    expect(screen.getByTestId('task-detail')).toBeInTheDocument()
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    cleanup()
    expect(useAppStore.getState().activeTaskId).toBe('previous')
  })
})

describe('ExecuteBacklogTable', () => {
  const groups = [
    backlogGroup({
      phaseTaskId: 'ph1', phaseTitle: 'Phase 1',
      tasks: [
        backlogTask({ taskId: 'e1', title: 'Long failure', lastError: 'x'.repeat(300), failedAttempts: 3, lastEngine: 'workflow', lastLinkStatus: 'failed' }),
        backlogTask({ taskId: 'e2', title: 'Silent failure', lastLinkStatus: 'failed' }),
        backlogTask({ taskId: 'e3', title: 'Odd engine', lastEngine: 'mystery' })
      ]
    })
  ]

  it('clamps long errors with a full-text title, explains missing detail, dashes unknown engines', () => {
    render(<ExecuteBacklogTable groups={groups} {...common} />)
    const long = screen.getByText('x'.repeat(300))
    expect(long.className).toContain('line-clamp-2')
    expect(long).toHaveAttribute('title', 'x'.repeat(300))
    expect(screen.getByText('Failed, no error detail recorded')).toBeInTheDocument()
    expect(screen.getByText('Workflow')).toBeInTheDocument()
    expect(screen.getByText('Phase 1')).toBeInTheDocument()
  })

  it('has no write actions: only task-title buttons and the plan link', () => {
    render(<ExecuteBacklogTable groups={groups} {...common} />)
    const names = screen.getAllByRole('button').map((b) => b.textContent)
    expect(names.some((n) => /run|execute|retry|status/i.test(n ?? ''))).toBe(false)
  })
})
