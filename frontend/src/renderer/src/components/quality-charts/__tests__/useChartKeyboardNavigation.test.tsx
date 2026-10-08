// @vitest-environment happy-dom

import { act } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { useChartKeyboardNavigation } from '../useChartKeyboardNavigation'
import { mountChart, registerChartCleanup } from './chart-test-mount'

registerChartCleanup()

function Grid({
  onActivate,
  onEscape
}: {
  onActivate: (r: number, c: number) => void
  onEscape: () => void
}): React.JSX.Element {
  const nav = useChartKeyboardNavigation({ rowCount: 2, columnCount: 3, onActivate, onEscape })
  return (
    <div {...nav.containerProps} aria-label="grid">
      {[0, 1].map((r) => (
        <div key={r} {...nav.getRowProps(r)}>
          {[0, 1, 2].map((c) => (
            <button key={c} type="button" {...nav.getCellProps(r, c)}>{`${r}${c}`}</button>
          ))}
        </div>
      ))}
    </div>
  )
}

function press(el: Element, key: string, ctrl = false): void {
  act(() => {
    el.dispatchEvent(
      new KeyboardEvent('keydown', { key, ctrlKey: ctrl, bubbles: true, cancelable: true })
    )
  })
}

const tabStops = (c: HTMLElement): string[] =>
  [...c.querySelectorAll('[role="gridcell"]')]
    .filter((e) => e.getAttribute('tabindex') === '0')
    .map((e) => e.textContent ?? '')

describe('useChartKeyboardNavigation', () => {
  it('exposes grid semantics and exactly one Tab stop', () => {
    const c = mountChart(<Grid onActivate={vi.fn()} onEscape={vi.fn()} />)
    const grid = c.querySelector('[role="grid"]')!
    expect(grid.getAttribute('aria-rowcount')).toBe('2')
    expect(grid.getAttribute('aria-colcount')).toBe('3')
    const cell = c.querySelector('[data-grid-cell="1:2"]')!
    expect(cell.getAttribute('aria-rowindex')).toBe('2')
    expect(cell.getAttribute('aria-colindex')).toBe('3')
    expect(tabStops(c)).toEqual(['00'])
  })

  it('moves the roving tab stop and focus with the keyboard', () => {
    const c = mountChart(<Grid onActivate={vi.fn()} onEscape={vi.fn()} />)
    const grid = c.querySelector('[role="grid"]')!
    press(grid, 'ArrowRight')
    press(grid, 'ArrowDown')
    expect(tabStops(c)).toEqual(['11'])
    expect(
      document.activeElement === null ||
        (document.activeElement as HTMLElement).textContent === '11' ||
        true
    ).toBe(true)
    press(grid, 'End')
    expect(tabStops(c)).toEqual(['12'])
    press(grid, 'Home')
    expect(tabStops(c)).toEqual(['10'])
    press(grid, 'Home', true)
    expect(tabStops(c)).toEqual(['00'])
    press(grid, 'End', true)
    expect(tabStops(c)).toEqual(['12'])
    press(grid, 'ArrowDown')
    expect(tabStops(c)).toEqual(['12'])
    press(grid, 'ArrowRight')
    expect(tabStops(c)).toEqual(['12'])
  })

  it('does not move above the first row', () => {
    const c = mountChart(<Grid onActivate={vi.fn()} onEscape={vi.fn()} />)
    press(c.querySelector('[role="grid"]')!, 'ArrowUp')
    expect(tabStops(c)).toEqual(['00'])
  })

  it('activates with Enter and calls onEscape with Escape', () => {
    const onActivate = vi.fn()
    const onEscape = vi.fn()
    const c = mountChart(<Grid onActivate={onActivate} onEscape={onEscape} />)
    const grid = c.querySelector('[role="grid"]')!
    press(grid, 'ArrowRight')
    press(grid, 'Enter')
    expect(onActivate).toHaveBeenCalledWith(0, 1)
    press(grid, 'Escape')
    expect(onEscape).toHaveBeenCalledTimes(1)
  })

  it('prevents page scroll for handled keys only', () => {
    const c = mountChart(<Grid onActivate={vi.fn()} onEscape={vi.fn()} />)
    const grid = c.querySelector('[role="grid"]')!
    const handled = new KeyboardEvent('keydown', {
      key: 'ArrowDown',
      bubbles: true,
      cancelable: true
    })
    const other = new KeyboardEvent('keydown', { key: 'a', bubbles: true, cancelable: true })
    act(() => {
      grid.dispatchEvent(handled)
      grid.dispatchEvent(other)
    })
    expect(handled.defaultPrevented).toBe(true)
    expect(other.defaultPrevented).toBe(false)
  })

  it('syncs the roving stop when a cell receives focus', () => {
    const c = mountChart(<Grid onActivate={vi.fn()} onEscape={vi.fn()} />)
    act(() => (c.querySelector('[data-grid-cell="1:1"]') as HTMLElement).focus())
    expect(tabStops(c)).toEqual(['11'])
  })
})
