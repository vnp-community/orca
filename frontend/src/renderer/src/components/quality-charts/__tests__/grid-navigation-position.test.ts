import { describe, expect, it } from 'vitest'
import { nextGridPosition, type GridLayout } from '../grid-navigation-position'

const dense: GridLayout = { kind: 'dense', rowCount: 3, columnCount: 4 }

describe('nextGridPosition (dense)', () => {
  it('moves with arrows and clamps at edges', () => {
    expect(nextGridPosition({ row: 0, col: 0 }, 'ArrowUp', dense)).toEqual({ row: 0, col: 0 })
    expect(nextGridPosition({ row: 0, col: 0 }, 'ArrowLeft', dense)).toEqual({ row: 0, col: 0 })
    expect(nextGridPosition({ row: 0, col: 0 }, 'ArrowRight', dense)).toEqual({ row: 0, col: 1 })
    expect(nextGridPosition({ row: 2, col: 3 }, 'ArrowDown', dense)).toEqual({ row: 2, col: 3 })
    expect(nextGridPosition({ row: 1, col: 1 }, 'ArrowDown', dense)).toEqual({ row: 2, col: 1 })
  })

  it('handles Home/End and Ctrl+Home/End', () => {
    expect(nextGridPosition({ row: 1, col: 2 }, 'Home', dense)).toEqual({ row: 1, col: 0 })
    expect(nextGridPosition({ row: 1, col: 2 }, 'End', dense)).toEqual({ row: 1, col: 3 })
    expect(nextGridPosition({ row: 1, col: 2 }, 'CtrlHome', dense)).toEqual({ row: 0, col: 0 })
    expect(nextGridPosition({ row: 1, col: 2 }, 'CtrlEnd', dense)).toEqual({ row: 2, col: 3 })
  })

  it('is a no-op for empty grids', () => {
    const empty: GridLayout = { kind: 'dense', rowCount: 0, columnCount: 0 }
    expect(nextGridPosition({ row: 0, col: 0 }, 'ArrowRight', empty)).toEqual({ row: 0, col: 0 })
  })
})

describe('nextGridPosition (sparse)', () => {
  const sparse: GridLayout = {
    kind: 'sparse',
    cells: [
      { row: 0, col: 3 },
      { row: 0, col: 1 },
      { row: 2, col: 2 },
      { row: 2, col: 5 }
    ]
  }
  it('jumps between existing cells', () => {
    expect(nextGridPosition({ row: 0, col: 1 }, 'ArrowRight', sparse)).toEqual({ row: 0, col: 3 })
    expect(nextGridPosition({ row: 0, col: 3 }, 'ArrowRight', sparse)).toEqual({ row: 0, col: 3 })
    expect(nextGridPosition({ row: 0, col: 3 }, 'ArrowDown', sparse)).toEqual({ row: 2, col: 2 })
    expect(nextGridPosition({ row: 2, col: 5 }, 'ArrowUp', sparse)).toEqual({ row: 0, col: 3 })
    expect(nextGridPosition({ row: 2, col: 2 }, 'End', sparse)).toEqual({ row: 2, col: 5 })
    expect(nextGridPosition({ row: 2, col: 2 }, 'CtrlHome', sparse)).toEqual({ row: 0, col: 1 })
    expect(nextGridPosition({ row: 0, col: 1 }, 'CtrlEnd', sparse)).toEqual({ row: 2, col: 5 })
  })
})
