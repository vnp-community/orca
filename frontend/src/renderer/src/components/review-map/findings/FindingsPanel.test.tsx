// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { FINDINGS_MIXED } from '../../../test-support/contract-findings-fixtures'

const findingsHook = vi.hoisted(() => ({ value: null as unknown }))
const dismissalHook = vi.hoisted(() => ({ value: null as unknown }))
const filtersSeen = vi.hoisted(() => ({ calls: [] as unknown[] }))
vi.mock('../../../hooks/useCodeIntelFindings', () => ({
  useCodeIntelFindings: (_w: string, _e: unknown, filters: unknown) => {
    filtersSeen.calls.push(filters)
    return findingsHook.value
  }
}))
vi.mock('../../../hooks/useFindingDismissal', () => ({
  useFindingDismissal: () => dismissalHook.value,
  FINDING_TEXT_MAX: 500
}))
const actions = vi.hoisted(() => ({
  setReviewLens: vi.fn(),
  selectReviewSymbol: vi.fn(),
  setErdService: vi.fn(),
  selectErdTable: vi.fn()
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (s: typeof actions) => unknown) => selector(actions)
}))

const summary = vi.hoisted(() => vi.fn())
vi.mock('@/lib/review-telemetry', () => ({ trackReviewFindingsSummary: summary }))
vi.mock('../notes/ReviewNoteButton', () => ({
  ReviewNoteButton: (p: { anchor: { kind: string; findingKey: string; filePath: string; startLine?: number } }) => (
    <button type="button" data-testid="note-btn" data-anchor={JSON.stringify(p.anchor)}>
      note
    </button>
  )
}))

import { FindingsPanel } from './FindingsPanel'

function setFindings(over: Record<string, unknown> = {}): void {
  findingsHook.value = {
    status: 'success',
    error: null,
    findings: FINDINGS_MIXED,
    dismissedCount: 2,
    indexFreshness: { state: 'fresh', dirtyFiles: 0, unindexedFiles: [], generatedAt: '2026-10-07' },
    stale: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    nextPageError: null,
    loadMore: vi.fn(),
    reload: vi.fn(),
    setOverride: vi.fn(),
    clearOverride: vi.fn(),
    ...over
  }
}

const dismiss = vi.fn(async () => ({ ok: true }))
const restore = vi.fn(async () => ({ ok: true }))

beforeEach(() => {
  setFindings()
  dismissalHook.value = {
    dismiss,
    resolve: vi.fn(async () => ({ ok: true })),
    restore,
    pendingKeys: new Set<string>(),
    errorsByKey: {},
    clearError: vi.fn()
  }
  filtersSeen.calls.length = 0
  summary.mockClear()
})
afterEach(cleanup)

const baseProps = {
  worktreeId: 'wt',
  environmentId: null,
  changedFiles: new Set(['svc/handlers/order.go']),
  graphSymbolKeys: null,
  availableLensIds: new Set(['structure', 'erd']),
  onOpenDiff: vi.fn(),
  onOpenFile: vi.fn()
}

describe('FindingsPanel', () => {
  it('defaults to scope=changed and only findings introduced by this change', () => {
    render(<FindingsPanel {...baseProps} />)
    expect(filtersSeen.calls[0]).toMatchObject({ scope: 'changed', includeDismissed: false })
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    fireEvent.click(screen.getByLabelText('Only from this change'))
    expect(screen.getAllByRole('listitem')).toHaveLength(4)
  })

  it('asks the server for dismissed rows when "show dismissed" is on and shows the count', () => {
    render(<FindingsPanel {...baseProps} />)
    fireEvent.click(screen.getByLabelText('Show dismissed (2)'))
    expect(filtersSeen.calls.at(-1)).toMatchObject({ includeDismissed: true })
  })

  it('opens the diff for a changed file and the file otherwise', () => {
    const onOpenDiff = vi.fn()
    const onOpenFile = vi.fn()
    render(<FindingsPanel {...baseProps} onOpenDiff={onOpenDiff} onOpenFile={onOpenFile} />)
    fireEvent.click(screen.getAllByText('View diff')[0])
    expect(onOpenDiff).toHaveBeenCalledWith('svc/handlers/order.go', 40)
    fireEvent.click(screen.getByLabelText('Only from this change'))
    fireEvent.click(screen.getAllByText('Open file')[0])
    expect(onOpenFile).toHaveBeenCalled()
  })

  it('gives each finding with a location a Note action anchored to the finding', () => {
    render(<FindingsPanel {...baseProps} />)
    const anchor = JSON.parse(screen.getAllByTestId('note-btn')[0].getAttribute('data-anchor')!)
    expect(anchor).toMatchObject({ kind: 'finding', filePath: 'svc/handlers/order.go', startLine: 40 })
    expect(anchor.label).toContain('Layer violation')
  })

  it('renders an empty state that makes no safety claim, plus index info', () => {
    setFindings({ findings: [] })
    render(<FindingsPanel {...baseProps} />)
    expect(screen.getByText(/No problems were found in the indexed scope/)).toBeTruthy()
    expect(screen.queryByText(/safe|all clear/i)).toBeNull()
  })

  it('shows loading and an inline retry on error', () => {
    setFindings({ status: 'loading', findings: [] })
    const { rerender } = render(<FindingsPanel {...baseProps} />)
    expect(screen.getByRole('status').textContent).toContain('Loading findings')
    const reload = vi.fn()
    setFindings({ status: 'error', error: { kind: 'tool-failed' }, reload })
    rerender(<FindingsPanel {...baseProps} />)
    fireEvent.click(screen.getByText('Retry'))
    expect(reload).toHaveBeenCalled()
  })

  it('offers Load more while the server has another page', () => {
    const loadMore = vi.fn()
    setFindings({ hasNextPage: true, loadMore })
    render(<FindingsPanel {...baseProps} />)
    fireEvent.click(screen.getByText('Load more'))
    expect(loadMore).toHaveBeenCalled()
  })

  it('shows a stale chip when the index is behind', () => {
    setFindings({ indexFreshness: { state: 'behind', dirtyFiles: 0, unindexedFiles: [], generatedAt: 'x' } })
    render(<FindingsPanel {...baseProps} />)
    expect(screen.getByText('Data may be out of date')).toBeTruthy()
  })

  it('j moves the active row and r reopens a dismissed one', () => {
    const dismissed = {
      ...FINDINGS_MIXED[0],
      findingKey: 'dismissed-1',
      dismissed: { by: 'u', at: 'x', reason: 'later', disposition: 'ignored' as const }
    }
    setFindings({ findings: [dismissed] })
    render(<FindingsPanel {...baseProps} />)
    fireEvent.click(screen.getByLabelText('Show dismissed (2)'))
    const region = screen.getByLabelText('Structural findings')
    fireEvent.keyDown(region, { key: 'j' })
    fireEvent.keyDown(region, { key: 'r' })
    expect(restore).toHaveBeenCalledWith(expect.objectContaining({ findingKey: 'dismissed-1' }))
  })

  it('ignores shortcut keys typed in the search box', () => {
    setFindings({ findings: [{ ...FINDINGS_MIXED[0], dismissed: { by: 'u', at: 'x', reason: 'later', disposition: 'ignored' as const } }] })
    restore.mockClear()
    render(<FindingsPanel {...baseProps} />)
    fireEvent.click(screen.getByLabelText('Show dismissed (2)'))
    const search = screen.getByLabelText('Search findings')
    fireEvent.keyDown(search, { key: 'j' })
    fireEvent.keyDown(search, { key: 'r' })
    expect(restore).not.toHaveBeenCalled()
  })

  it('sends one findings summary per mount, as counts only', () => {
    const { rerender } = render(<FindingsPanel {...baseProps} />)
    rerender(<FindingsPanel {...baseProps} />)
    expect(summary).toHaveBeenCalledTimes(1)
    expect(Object.keys(summary.mock.calls[0][0] as object).sort()).toEqual([
      'dismissed',
      'resolved',
      'shown',
      'waived'
    ])
  })
})
