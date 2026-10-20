// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { UseRequestActivityResult } from '../../../hooks/useRequestActivity'
import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'
import type { RequestTypeHistoryEntry } from '../../../../../shared/request-types'

const hook = vi.fn()
vi.mock('../../../hooks/useRequestActivity', () => ({
  useRequestActivity: (...args: unknown[]) => hook(...args)
}))

import { RequestActivityTab } from './RequestActivityTab'

const loadMore = vi.fn()
const refetch = vi.fn()
const retryTypeHistory = vi.fn()

const entry = (over: Partial<RequestActivityEntry> = {}): RequestActivityEntry => ({
  id: 'e1',
  kind: 'task.failed',
  occurredAt: new Date().toISOString(),
  actorKind: 'agent',
  summary: 'backend summary',
  severity: 'attention',
  attempt: 2,
  details: {
    taskTitle: 'Fix login',
    taskNumber: 3,
    attempt: 2,
    maxAttempts: 3,
    cause: 'timed_out',
    error: 'deadline exceeded'
  },
  ...over
})

function state(over: Partial<UseRequestActivityResult> = {}): UseRequestActivityResult {
  return {
    entries: [],
    hasMore: false,
    isLoading: false,
    isLoadingMore: false,
    error: null,
    unsupported: false,
    loadMore,
    refetch,
    ...over
  }
}

const typeHistory: RequestTypeHistoryEntry[] = [
  {
    id: 'h1',
    requestId: 'r1',
    fromType: 'bug',
    toType: 'task',
    reason: 'Reclassified as work item',
    actorKind: 'ai',
    occurredAt: '2026-10-01T00:00:00Z'
  }
]

function renderTab(active = true) {
  return render(
    <RequestActivityTab
      requestId="r1"
      active={active}
      typeHistory={typeHistory}
      typeHistoryLoading={false}
      typeHistoryError={null}
      onRetryTypeHistory={retryTypeHistory}
    />
  )
}

const lastCall = () => hook.mock.calls.at(-1) as [string, { enabled: boolean; filter: object }]

beforeEach(() => {
  hook.mockReturnValue(state())
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('RequestActivityTab', () => {
  it('reads the feed only while the tab is active', () => {
    renderTab(false)
    expect(lastCall()[1].enabled).toBe(false)
    expect(screen.queryByTestId('request-activity-tab')).toBeNull()
    cleanup()
    renderTab(true)
    expect(lastCall()[0]).toBe('r1')
    expect(lastCall()[1].enabled).toBe(true)
  })

  it('shows a skeleton on first load', () => {
    hook.mockReturnValue(state({ isLoading: true }))
    renderTab()
    expect(screen.getByTestId('activity-loading')).toHaveAttribute('aria-busy', 'true')
    expect(screen.queryByTestId('activity-empty')).toBeNull()
  })

  it('shows the empty state', () => {
    renderTab()
    expect(screen.getByTestId('activity-empty')).toHaveTextContent('No activity yet')
  })

  it('renders entries grouped by day with icon, words, actor, attempt and detail', () => {
    hook.mockReturnValue(
      state({
        entries: [
          entry(),
          entry({
            id: 'e2',
            kind: 'approval.decided',
            actorKind: 'user',
            severity: 'info',
            subjectType: 'plan',
            attempt: undefined,
            details: { result: 'approved', comment: 'Ship it' }
          }),
          entry({
            id: 'e3',
            kind: 'task.failed',
            severity: 'error',
            occurredAt: '2026-10-01T10:00:00Z',
            details: { taskTitle: 'Old task', attempt: 3, maxAttempts: 3 },
            attempt: 3
          })
        ]
      })
    )
    renderTab()
    const groups = screen.getAllByTestId('activity-day-group')
    expect(groups).toHaveLength(2)
    expect(within(groups[0]).getByRole('heading', { name: 'Today' })).toBeInTheDocument()
    expect(within(groups[0]).getByRole('list')).toBeInTheDocument()

    const rows = screen.getAllByTestId('activity-entry')
    expect(rows[0]).toHaveTextContent('#TG-3 Fix login failed')
    expect(rows[0]).toHaveTextContent('Attempt 2 of 3')
    expect(rows[0]).toHaveTextContent('Timed out: deadline exceeded')
    expect(rows[0]).toHaveTextContent('Agent')
    expect(within(rows[0]).getByTestId('activity-entry-severity')).toHaveTextContent(
      'Needs attention'
    )
    expect(rows[1]).toHaveTextContent('The plan was approved')
    expect(rows[1]).toHaveTextContent('Ship it')
    expect(rows[1]).toHaveTextContent('User')
    expect(within(rows[1]).queryByTestId('activity-entry-severity')).toBeNull()
    // Errors carry words and an icon, not just a colour.
    expect(within(rows[2]).getByTestId('activity-entry-severity')).toHaveTextContent('Stopped')
    expect(rows[2].querySelector('svg[aria-hidden="true"]')).not.toBeNull()
  })

  it('renders an unknown kind with its summary', () => {
    hook.mockReturnValue(
      state({
        entries: [
          entry({
            id: 'x',
            kind: 'thing.happened',
            summary: 'Something new happened',
            severity: 'info',
            attempt: undefined,
            details: {}
          })
        ]
      })
    )
    renderTab()
    expect(screen.getByTestId('activity-entry')).toHaveTextContent('Something new happened')
  })

  it('renders a revision request as one plain line that never carries the reviewer text', () => {
    hook.mockReturnValue(
      state({
        entries: [
          entry({
            id: 'run-1:revision',
            kind: 'analysis.revision_requested',
            actorKind: 'user',
            severity: 'info',
            subjectType: 'solution',
            subjectId: 's-2',
            summary: 'Requested changes to the analysis',
            attempt: undefined,
            details: { solutionId: 's-2' }
          })
        ]
      })
    )
    renderTab()
    const row = screen.getByTestId('activity-entry')
    expect(row).toHaveTextContent('Requested changes to the analysis')
    expect(row).toHaveTextContent('User')
    expect(within(row).queryByTestId('activity-entry-severity')).toBeNull()
  })

  it('renders a plan change request as one plain line, with who asked, no severity and no text', () => {
    hook.mockReturnValue(
      state({
        entries: [
          entry({
            id: 'run-3:revision',
            kind: 'plan.revision_requested',
            actorKind: 'user',
            severity: 'info',
            subjectType: 'plan',
            subjectId: 'plan-1',
            summary: 'Requested changes to the plan',
            attempt: undefined,
            details: { feedback: 'secret reviewer text' }
          })
        ]
      })
    )
    renderTab()
    const row = screen.getByTestId('activity-entry')
    expect(row).toHaveTextContent('Requested changes to the plan')
    expect(row).toHaveTextContent('User')
    expect(row).not.toHaveTextContent('secret reviewer text')
    expect(within(row).queryByTestId('activity-entry-severity')).toBeNull()
  })

  it('passes filter changes to the hook', () => {
    renderTab()
    expect(lastCall()[1].filter).toEqual({})

    fireEvent.click(screen.getByRole('radio', { name: 'Approvals' }))
    expect(lastCall()[1].filter).toEqual({ kinds: ['approval.requested', 'approval.decided'] })

    fireEvent.click(screen.getByRole('button', { name: /Needs attention only/ }))
    expect(lastCall()[1].filter).toEqual({
      kinds: ['approval.requested', 'approval.decided'],
      minSeverity: 'attention'
    })

    fireEvent.click(screen.getByRole('radio', { name: 'AI' }))
    expect(lastCall()[1].filter).toMatchObject({ actorKind: 'ai', minSeverity: 'attention' })

    fireEvent.click(screen.getByRole('radio', { name: 'All' }))
    fireEvent.click(screen.getByRole('button', { name: /Needs attention only/ }))
    fireEvent.click(screen.getByRole('radio', { name: 'Anyone' }))
    expect(lastCall()[1].filter).toEqual({})
  })

  it('says the filter matched nothing when a filter is on', () => {
    renderTab()
    fireEvent.click(screen.getByRole('radio', { name: 'Tasks' }))
    expect(screen.getByTestId('activity-empty')).toHaveTextContent(
      'No activity matches these filters.'
    )
  })

  it('loads more on demand and disables the button while loading', () => {
    hook.mockReturnValue(state({ entries: [entry()], hasMore: true }))
    renderTab()
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(loadMore).toHaveBeenCalledTimes(1)
    cleanup()
    hook.mockReturnValue(state({ entries: [entry()], hasMore: true, isLoadingMore: true }))
    renderTab()
    expect(screen.getByRole('button', { name: 'Loading…' })).toBeDisabled()
  })

  it('hides Load more on the last page', () => {
    hook.mockReturnValue(state({ entries: [entry()], hasMore: false }))
    renderTab()
    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull()
  })

  it('shows the error with Retry that refetches, and keeps the filters', () => {
    hook.mockReturnValue(state({ error: 'network' }))
    renderTab()
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('activity-filter-bar')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('falls back to the type-change list on an older runtime, without an error', () => {
    hook.mockReturnValue(state({ unsupported: true }))
    renderTab()
    expect(screen.getByTestId('request-history-list')).toHaveTextContent(
      'Reclassified as work item'
    )
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByTestId('activity-filter-bar')).toBeNull()
  })
})
