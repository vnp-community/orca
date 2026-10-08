// @vitest-environment happy-dom
import type React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import {
  makeQualityFinding,
  makeQualityFindings
} from '../../../../test-support/quality-finding-fixtures'
import { TooltipProvider } from '../../../ui/tooltip'
import { QualityFindingsList } from './QualityFindingsList'
import type { QualityFindingsListProps } from './QualityFindingsList'

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(400)
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(600)
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  )
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const renderList = (ui: React.ReactElement) => {
  const view = render(<TooltipProvider>{ui}</TooltipProvider>)
  return {
    ...view,
    rerender: (next: React.ReactElement) => view.rerender(<TooltipProvider>{next}</TooltipProvider>)
  }
}

function props(patch: Partial<QualityFindingsListProps> = {}): QualityFindingsListProps {
  return {
    items: [],
    status: 'ready',
    total: 0,
    truncated: false,
    hasMore: false,
    capped: false,
    loadingMore: false,
    outsideScopeCount: 0,
    emptyReason: 'none',
    selectedFingerprint: null,
    onLoadMore: vi.fn(),
    onRetry: vi.fn(),
    ...patch
  }
}

describe('QualityFindingsList', () => {
  it('renders every row below the virtualization threshold', () => {
    renderList(<QualityFindingsList {...props({ items: makeQualityFindings(10), total: 10 })} />)
    expect(screen.getAllByRole('row')).toHaveLength(10)
    expect(screen.getByText('Showing 10 of 10')).toBeTruthy()
  })

  it('virtualizes above 50 rows (fewer DOM rows than items)', () => {
    renderList(<QualityFindingsList {...props({ items: makeQualityFindings(400), total: 400 })} />)
    const rows = screen.getAllByRole('row')
    expect(rows.length).toBeGreaterThan(0)
    expect(rows.length).toBeLessThan(400)
  })

  it('shows message HTML as text and an unknown rule id verbatim', () => {
    const item = makeQualityFinding({
      message: '<img src=x onerror=alert(1)>',
      ruleId: 'Weird:Rule/9'
    })
    const { container } = renderList(
      <QualityFindingsList {...props({ items: [item], total: 1 })} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(screen.getAllByText('<img src=x onerror=alert(1)>').length).toBeGreaterThan(0)
    expect(screen.getByText('Weird:Rule/9')).toBeTruthy()
  })

  it('states X/Y, outside-scope and cap notices, and loads more', () => {
    const onLoadMore = vi.fn()
    renderList(
      <QualityFindingsList
        {...props({
          items: makeQualityFindings(5),
          total: 1203,
          outsideScopeCount: 12,
          hasMore: true,
          onLoadMore
        })}
      />
    )
    expect(screen.getByText('Showing 5 of 1203')).toBeTruthy()
    expect(screen.getByText(/12 outside the changed scope/)).toBeTruthy()
    fireEvent.click(screen.getByText('Load more'))
    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })

  it('reports the cap instead of offering more pages', () => {
    renderList(
      <QualityFindingsList
        {...props({ items: makeQualityFindings(5), total: 9000, hasMore: true, capped: true })}
      />
    )
    expect(screen.getByText(/first 5,000/)).toBeTruthy()
    expect(screen.queryByText('Load more')).toBeNull()
  })

  it('explains empty states and shows an inline retry on error', () => {
    const onRetry = vi.fn()
    const { rerender } = renderList(<QualityFindingsList {...props({ emptyReason: 'no-run' })} />)
    expect(screen.getByText(/No checks have run yet/)).toBeTruthy()
    rerender(<QualityFindingsList {...props({ emptyReason: 'none' })} />)
    expect(screen.getByText(/scope of the checks that ran/)).toBeTruthy()
    rerender(<QualityFindingsList {...props({ emptyReason: 'filtered' })} />)
    expect(screen.getByText(/match the current filters/)).toBeTruthy()
    rerender(<QualityFindingsList {...props({ status: 'error', onRetry })} />)
    fireEvent.click(screen.getByText('Retry'))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('opens on Enter and by button, labelling out-of-scope rows as file opens', () => {
    const onOpen = vi.fn()
    const items = [
      makeQualityFinding({ fingerprint: 'a' }),
      makeQualityFinding({ fingerprint: 'b', inScope: false })
    ]
    renderList(<QualityFindingsList {...props({ items, total: 2, onOpen })} />)
    expect(screen.getByText('View diff')).toBeTruthy()
    expect(screen.getByText('Open file')).toBeTruthy()
    fireEvent.keyDown(screen.getAllByRole('row')[0], { key: 'Enter' })
    expect(onOpen).toHaveBeenCalledWith(items[0])
  })

  it('highlights the selected fingerprint and shows waiver details', () => {
    const waiver = { by: 'ana', reason: 'legacy', expiresAt: '2026-11-01T00:00:00Z' }
    const items = [
      makeQualityFinding({ fingerprint: 'a', waiver }),
      makeQualityFinding({ fingerprint: 'b' })
    ]
    renderList(<QualityFindingsList {...props({ items, total: 2, selectedFingerprint: 'b' })} />)
    expect(screen.getAllByRole('row')[1].getAttribute('data-selected')).toBe('true')
    expect(screen.getByText(/Waived by ana until/)).toBeTruthy()
  })
})
