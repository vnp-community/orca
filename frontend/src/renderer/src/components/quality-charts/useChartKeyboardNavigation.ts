import { useCallback, useRef, useState } from 'react'
import {
  nextGridPosition,
  type GridLayout,
  type GridNavigationKey,
  type GridPosition
} from './grid-navigation-position'

type Options = {
  rowCount: number
  columnCount: number
  /** Sparse grids (e.g. dependency matrix) list only the cells that exist. */
  cells?: readonly GridPosition[]
  onActivate?: (row: number, col: number) => void
  onEscape?: () => void
}

function keyOf(event: React.KeyboardEvent): GridNavigationKey | null {
  const ctrl = event.ctrlKey || event.metaKey
  switch (event.key) {
    case 'ArrowRight':
    case 'ArrowLeft':
    case 'ArrowDown':
    case 'ArrowUp':
      return event.key
    case 'Home':
      return ctrl ? 'CtrlHome' : 'Home'
    case 'End':
      return ctrl ? 'CtrlEnd' : 'End'
    default:
      return null
  }
}

/**
 * One Tab stop for the whole grid (roving tabindex), arrow/Home/End navigation, Enter to activate,
 * Escape to leave. Why: hundreds of cells must not become hundreds of Tab stops.
 */
export function useChartKeyboardNavigation({
  rowCount,
  columnCount,
  cells,
  onActivate,
  onEscape
}: Options) {
  const containerRef = useRef<HTMLDivElement | SVGSVGElement | null>(null)
  const layout: GridLayout = cells
    ? { kind: 'sparse', cells }
    : { kind: 'dense', rowCount, columnCount }
  const first: GridPosition =
    cells && cells.length > 0
      ? [...cells].sort((a, b) => a.row - b.row || a.col - b.col)[0]
      : { row: 0, col: 0 }
  const [active, setActive] = useState<GridPosition>(first)

  const exists = cells
    ? cells.some((c) => c.row === active.row && c.col === active.col)
    : active.row < rowCount && active.col < columnCount
  const activeCell = exists ? active : first

  const focusCell = useCallback((position: GridPosition) => {
    const target = containerRef.current?.querySelector<HTMLElement | SVGElement>(
      `[data-grid-cell="${position.row}:${position.col}"]`
    )
    target?.focus()
  }, [])

  const onKeyDown = (event: React.KeyboardEvent): void => {
    const nav = keyOf(event)
    if (nav) {
      event.preventDefault()
      const next = nextGridPosition(activeCell, nav, layout)
      setActive(next)
      focusCell(next)
      return
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      onActivate?.(activeCell.row, activeCell.col)
    } else if (event.key === 'Escape') {
      onEscape?.()
      ;(containerRef.current as HTMLElement | null)?.focus?.()
    }
  }

  const containerProps = {
    ref: containerRef as React.RefObject<never>,
    role: 'grid' as const,
    'aria-rowcount': rowCount,
    'aria-colcount': columnCount,
    tabIndex: -1,
    onKeyDown
  }

  const getCellProps = (row: number, col: number) => ({
    role: 'gridcell' as const,
    'aria-rowindex': row + 1,
    'aria-colindex': col + 1,
    tabIndex: row === activeCell.row && col === activeCell.col ? 0 : -1,
    'data-grid-cell': `${row}:${col}`,
    onFocus: () =>
      setActive((prev) => (prev.row === row && prev.col === col ? prev : { row, col })),
    onClick: () => {
      setActive({ row, col })
      onActivate?.(row, col)
    }
  })

  const getRowProps = (row: number) => ({ role: 'row' as const, 'aria-rowindex': row + 1 })

  return { activeCell, getCellProps, getRowProps, containerProps }
}
