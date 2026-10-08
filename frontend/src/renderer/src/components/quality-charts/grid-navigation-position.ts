export type GridPosition = { row: number; col: number }

export type GridLayout =
  | { kind: 'dense'; rowCount: number; columnCount: number }
  | { kind: 'sparse'; cells: readonly GridPosition[] }

export type GridNavigationKey =
  | 'ArrowRight'
  | 'ArrowLeft'
  | 'ArrowDown'
  | 'ArrowUp'
  | 'Home'
  | 'End'
  | 'CtrlHome'
  | 'CtrlEnd'

function clamp(value: number, max: number): number {
  return Math.min(Math.max(0, value), Math.max(0, max))
}

function sortedCells(cells: readonly GridPosition[]): GridPosition[] {
  return [...cells].sort((a, b) => a.row - b.row || a.col - b.col)
}

/** Pure APG-style grid movement; sparse layouts jump between existing cells only. */
export function nextGridPosition(
  current: GridPosition,
  key: GridNavigationKey,
  layout: GridLayout
): GridPosition {
  if (layout.kind === 'dense') {
    const lastRow = layout.rowCount - 1
    const lastCol = layout.columnCount - 1
    if (lastRow < 0 || lastCol < 0) {
      return current
    }
    switch (key) {
      case 'ArrowRight':
        return { row: current.row, col: clamp(current.col + 1, lastCol) }
      case 'ArrowLeft':
        return { row: current.row, col: clamp(current.col - 1, lastCol) }
      case 'ArrowDown':
        return { row: clamp(current.row + 1, lastRow), col: current.col }
      case 'ArrowUp':
        return { row: clamp(current.row - 1, lastRow), col: current.col }
      case 'Home':
        return { row: current.row, col: 0 }
      case 'End':
        return { row: current.row, col: lastCol }
      case 'CtrlHome':
        return { row: 0, col: 0 }
      case 'CtrlEnd':
        return { row: lastRow, col: lastCol }
    }
  }

  const cells = sortedCells(layout.cells)
  if (cells.length === 0) {
    return current
  }
  const inRow = (row: number): GridPosition[] => cells.filter((c) => c.row === row)
  const rowCells = inRow(current.row)
  const at = rowCells.findIndex((c) => c.col === current.col)
  switch (key) {
    case 'ArrowRight':
      return rowCells[Math.min(rowCells.length - 1, at + 1)] ?? current
    case 'ArrowLeft':
      return rowCells[Math.max(0, at - 1)] ?? current
    case 'Home':
      return rowCells[0] ?? current
    case 'End':
      return rowCells.at(-1) ?? current
    case 'CtrlHome':
      return cells[0]
    case 'CtrlEnd':
      return cells.at(-1) ?? current
    case 'ArrowDown':
    case 'ArrowUp': {
      const rows = [...new Set(cells.map((c) => c.row))]
      const rowAt = rows.indexOf(current.row)
      const target = rows[key === 'ArrowDown' ? rowAt + 1 : rowAt - 1]
      if (target === undefined) {
        return current
      }
      // Why: land on the closest column so vertical moves feel like a grid.
      return inRow(target).reduce((best, c) =>
        Math.abs(c.col - current.col) < Math.abs(best.col - current.col) ? c : best
      )
    }
  }
}
