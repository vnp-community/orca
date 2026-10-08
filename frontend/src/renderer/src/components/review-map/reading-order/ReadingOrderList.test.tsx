// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { buildReadingOrderItems } from '../reading-order-model'
import { EMPTY_PROGRESS, makeOverlay, makeStep } from '../review-test-data'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { ReadingProgress } from '../review-wire-types'
import { ReadingOrderList, type ReadingOrderListProps } from './ReadingOrderList'
import { ReadingProgressBar } from './ReadingProgressBar'

afterEach(cleanup)

function props(over: Partial<ReadingOrderListProps> = {}): ReadingOrderListProps {
  const items = buildReadingOrderItems([makeStep(1), makeStep(2), makeStep(3)], [], [])
  return {
    items,
    changedFileCount: 3,
    progress: EMPTY_PROGRESS,
    saveStatus: 'saved',
    filterFiles: null,
    onClearFilter: vi.fn(),
    onSetSeen: vi.fn(),
    onSetGroupSeen: vi.fn(),
    onFocusChange: vi.fn(),
    onOpenDiff: vi.fn(),
    onRetrySave: vi.fn(),
    onRetryProgressLoad: vi.fn(),
    ...over
  }
}
const list = (): HTMLElement => screen.getByRole('listbox')

describe('ReadingOrderList', () => {
  it('renders a listbox with the active option and group header', () => {
    render(<ReadingOrderList {...props()} />)
    expect(screen.getAllByRole('option')).toHaveLength(4)
    expect(list().getAttribute('aria-activedescendant')).toBe('ro-group-__other__')
  })

  it('j moves the active row and reports focus; Enter opens the diff at the first hunk without marking read', () => {
    const p = props()
    render(<ReadingOrderList {...p} />)
    fireEvent.keyDown(list(), { key: 'j' })
    expect(p.onFocusChange).toHaveBeenCalledWith('s1')
    expect(screen.getAllByRole('option')[1].getAttribute('aria-selected')).toBe('true')
    fireEvent.keyDown(list(), { key: 'Enter' })
    expect(p.onOpenDiff).toHaveBeenCalledWith('src/f1.ts', 11)
    expect(p.onSetSeen).not.toHaveBeenCalled()
  })

  it('Space toggles read on the active step and flips back when already read', () => {
    const p = props()
    const { rerender } = render(<ReadingOrderList {...p} />)
    fireEvent.keyDown(list(), { key: 'j' })
    fireEvent.keyDown(list(), { key: ' ' })
    expect(p.onSetSeen).toHaveBeenLastCalledWith('s1', true)
    const seen: ReadingProgress = {
      version: 1,
      lastFocusedKey: null,
      entries: { s1: { state: 'seen', at: 1 } }
    }
    rerender(<ReadingOrderList {...p} progress={seen} />)
    fireEvent.keyDown(list(), { key: ' ' })
    expect(p.onSetSeen).toHaveBeenLastCalledWith('s1', false)
  })

  it('ArrowLeft on the group header collapses it by keyboard', () => {
    render(<ReadingOrderList {...props()} />)
    fireEvent.keyDown(list(), { key: 'ArrowLeft' })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    fireEvent.keyDown(list(), { key: 'ArrowRight' })
    expect(screen.getAllByRole('option')).toHaveLength(4)
  })

  it('does not react to keys typed in an input', () => {
    const p = props()
    const { container } = render(
      <div>
        <input data-testid="i" />
        <ReadingOrderList {...p} />
      </div>
    )
    fireEvent.keyDown(container.querySelector('input')!, { key: 'j' })
    expect(p.onFocusChange).not.toHaveBeenCalled()
  })

  it('progress is counted on the whole list while a filter hides rows', () => {
    const progress: ReadingProgress = {
      version: 1,
      lastFocusedKey: null,
      entries: { s1: { state: 'seen', at: 1 } }
    }
    render(
      <ReadingOrderList
        {...props({ progress, filterFiles: new Set(['src/f2.ts']), filterLabel: 'files' })}
      />
    )
    expect(screen.getByText('1 of 3 read')).toBeTruthy()
    expect(screen.getByTestId('reading-filter-notice').textContent).toContain('1 of 3')
    expect(screen.getAllByRole('option')).toHaveLength(2)
  })

  it('virtualizes long lists (500 steps render a window, not 500 rows)', () => {
    // happy-dom has no layout: give the scroll container a viewport for the virtualizer.
    const h = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')
    const w = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth')
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 })
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 320 })
    const steps = Array.from({ length: 500 }, (_, i) => makeStep(i + 1))
    render(<ReadingOrderList {...props({ items: buildReadingOrderItems(steps, [], []) })} />)
    const rendered = screen.getAllByRole('option').length
    expect(rendered).toBeGreaterThan(0)
    expect(rendered).toBeLessThan(100)
    if (h) {
      Object.defineProperty(HTMLElement.prototype, 'offsetHeight', h)
    }
    if (w) {
      Object.defineProperty(HTMLElement.prototype, 'offsetWidth', w)
    }
  })

  it('initial focus lands on lastFocusedKey, else the first unseen step', () => {
    const progress: ReadingProgress = {
      version: 1,
      lastFocusedKey: 's2',
      entries: { s1: { state: 'seen', at: 1 } }
    }
    render(<ReadingOrderList {...props({ progress, initialFocusKey: 's2' })} />)
    expect(list().getAttribute('aria-activedescendant')).toBe('ro-step-s2')
  })

  it('empty states do not guess the cause', () => {
    const { unmount } = render(<ReadingOrderList {...props({ items: [], changedFileCount: 0 })} />)
    expect(screen.getByText('No changes in this scope.')).toBeTruthy()
    unmount()
    render(<ReadingOrderList {...props({ items: [], changedFileCount: 4 })} />)
    expect(screen.getByText('No reading order could be built from the index.')).toBeTruthy()
  })

  it('a failed progress load shows a small retry and keeps the list usable', () => {
    const p = props({ progressLoadFailed: true })
    render(<ReadingOrderList {...p} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(p.onRetryProgressLoad).toHaveBeenCalled()
    expect(screen.getAllByRole('option').length).toBeGreaterThan(0)
  })
})

describe('ReadingProgressBar', () => {
  it('never says Saved while dirty or saving; error shows Not saved + Retry', () => {
    const onRetrySave = vi.fn()
    const { rerender } = render(
      <ReadingProgressBar seen={1} total={3} saveStatus="dirty" onRetrySave={onRetrySave} />
    )
    expect(screen.queryByText('Saved')).toBeNull()
    rerender(<ReadingProgressBar seen={1} total={3} saveStatus="error" onRetrySave={onRetrySave} />)
    expect(screen.getByText(/Not saved/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetrySave).toHaveBeenCalled()
    rerender(<ReadingProgressBar seen={1} total={3} saveStatus="saved" onRetrySave={onRetrySave} />)
    expect(screen.getByText('Saved')).toBeTruthy()
  })
  it('forbidden save is local-only without a retry button', () => {
    render(
      <ReadingProgressBar seen={0} total={2} saveStatus="error" localOnly onRetrySave={vi.fn()} />
    )
    expect(screen.getByText(/read-only access/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })
  it('announces the count politely after a delay', async () => {
    vi.useFakeTimers()
    render(<ReadingProgressBar seen={2} total={5} saveStatus="saved" onRetrySave={vi.fn()} />)
    await vi.advanceTimersByTimeAsync(500)
    expect(document.querySelector('[aria-live=polite]')?.textContent).toBe('2/5')
    vi.useRealTimers()
  })
})

describe('ReadingOrderList overlay marks', () => {
  it('marks untested symbols and files with a finding using icon + text label', () => {
    const sym = { key: 'k1', kind: 'function', name: 'f', filePath: 'src/f1.ts' }
    const steps = [makeStep(1, { symbols: [sym] }), makeStep(2)]
    const items = buildReadingOrderItems(steps, [], [])
    const overlay = makeOverlay({
      changedSymbols: [{ symbol: sym, changeKind: 'modified', tested: 'no' }],
      violations: [
        { findingKey: 'v', rule: 'r', severity: 'warn', file: 'src/f2.ts', status: 'touched' }
      ]
    })
    render(
      <TooltipProvider>
        <ReadingOrderList {...props({ items, overlay })} />
      </TooltipProvider>
    )
    const flags = screen.getAllByRole('img').map((e) => e.getAttribute('data-flag'))
    expect(flags).toEqual(['untested', 'violation'])
    expect(screen.getByRole('img', { name: 'No test found' })).toBeTruthy()
  })
  it('without overlay shows no marks', () => {
    render(<ReadingOrderList {...props()} />)
    expect(screen.queryByTestId('reading-row-flags')).toBeNull()
  })
})
