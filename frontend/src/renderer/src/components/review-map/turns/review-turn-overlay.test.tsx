// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, renderHook } from '@testing-library/react'

vi.mock('@xyflow/react', () => ({ Handle: () => null, Position: { Left: 'l', Right: 'r' } }))

import { turnOverlayDimmed, turnOverlayLabel } from '../review-overlay-model'
import { ImpactSymbolNode } from '../impact/ImpactSymbolNode'
import { setReviewTurnOverlay, useReviewTurnOverlay } from './review-turn-overlay-store'
import { compareTurns } from './turn-compare-model'

const compare = compareTurns(
  {
    files: [
      { p: 'a.ts', h: '1' },
      { p: 'b.ts', h: '1' }
    ],
    symbolKeys: ['s1'],
    overlayAvailable: true
  },
  {
    files: [
      { p: 'a.ts', h: '1' },
      { p: 'b.ts', h: '2' },
      { p: 'c.ts', h: '1' }
    ],
    symbolKeys: ['s1', 's2'],
    overlayAvailable: true
  }
)

afterEach(() => {
  cleanup()
  setReviewTurnOverlay('wt', null)
})

describe('turn overlay labels', () => {
  it('labels nodes by symbol first, then by file', () => {
    expect(turnOverlayLabel({ symbolKey: 's2', file: 'a.ts' }, compare)).toBe('new_in_turn')
    expect(turnOverlayLabel({ file: 'a.ts' }, compare)).toBe('unchanged_since')
    expect(turnOverlayLabel({ file: 'b.ts' }, compare)).toBe('changed_in_turn')
    expect(turnOverlayLabel({ file: 'zzz.ts' }, compare)).toBeNull()
    expect(turnOverlayLabel({ file: 'a.ts' }, null)).toBeNull()
  })

  it('dims only nodes unchanged since the previous turn', () => {
    expect(turnOverlayDimmed('unchanged_since')).toBe(true)
    expect(turnOverlayDimmed('new_in_turn')).toBe(false)
    expect(turnOverlayDimmed(null)).toBe(false)
  })

  it('publishes the comparison per worktree to subscribed canvases', () => {
    const { result } = renderHook(() => useReviewTurnOverlay('wt'))
    expect(result.current).toBeNull()
    act(() => setReviewTurnOverlay('wt', compare))
    expect(result.current).toBe(compare)
    expect(renderHook(() => useReviewTurnOverlay('other')).result.current).toBeNull()
    act(() => setReviewTurnOverlay('wt', null))
    expect(result.current).toBeNull()
  })

  it('draws the label on an impact node and dims unchanged ones', () => {
    const data = (turnLabel: string | null) => ({
      kind: 'symbol',
      symbol: { key: 's1', name: 'Foo', kind: 'function', filePath: 'a.ts' },
      flags: new Set(),
      selected: false,
      column: 1,
      actions: { onSelect: vi.fn(), onFocus: vi.fn(), onOpenDiff: vi.fn() },
      onExpand: vi.fn(),
      turnLabel
    })
    const props = (l: string | null) =>
      ({ data: data(l) }) as unknown as Parameters<typeof ImpactSymbolNode>[0]
    const { container, rerender } = render(<ImpactSymbolNode {...props('unchanged_since')} />)
    const root = container.firstElementChild as HTMLElement
    expect(root.dataset.turnLabel).toBe('unchanged_since')
    expect(root.className).toContain('opacity-50')
    expect(root.textContent).toContain('Unchanged since previous turn')
    rerender(<ImpactSymbolNode {...props('new_in_turn')} />)
    expect((container.firstElementChild as HTMLElement).className).not.toContain('opacity-50')
    expect(container.textContent).toContain('New in this turn')
    rerender(<ImpactSymbolNode {...props(null)} />)
    expect((container.firstElementChild as HTMLElement).dataset.turnLabel).toBeUndefined()
  })
})
