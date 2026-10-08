// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import React from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useRowListKeyboardNavigation } from './useRowListKeyboardNavigation'

type Row = { id: string }

function List({ items, onOpen }: { items: Row[]; onOpen: (r: Row) => void }): React.JSX.Element {
  const { containerProps, getRowProps, activeKey } = useRowListKeyboardNavigation({
    items, getKey: (r) => r.id, onOpen
  })
  return (
    <div data-testid="list" {...containerProps}>
      <input aria-label="search" />
      <span data-testid="active">{activeKey ?? 'none'}</span>
      {items.map((r) => (
        <div key={r.id} data-testid={`row-${r.id}`} {...getRowProps(r.id)}>
          <button>act-{r.id}</button>
        </div>
      ))}
    </div>
  )
}

const rows = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]
afterEach(cleanup)

const active = () => screen.getByTestId('active').textContent

describe('useRowListKeyboardNavigation', () => {
  it('moves with j/k, stops at the edges, and opens with Enter', () => {
    const onOpen = vi.fn()
    render(<List items={rows} onOpen={onOpen} />)
    const list = screen.getByTestId('list')
    fireEvent.keyDown(list, { key: 'j' })
    expect(active()).toBe('a')
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'j' })
    expect(active()).toBe('c')
    fireEvent.keyDown(list, { key: 'k' })
    expect(active()).toBe('b')
    fireEvent.keyDown(list, { key: 'Enter' })
    expect(onOpen).toHaveBeenCalledWith({ id: 'b' })
  })

  it('ignores typing targets, modifiers, and IME composition', () => {
    render(<List items={rows} onOpen={vi.fn()} />)
    fireEvent.keyDown(screen.getByLabelText('search'), { key: 'j' })
    expect(active()).toBe('none')
    const list = screen.getByTestId('list')
    fireEvent.keyDown(list, { key: 'j', ctrlKey: true })
    fireEvent.keyDown(list, { key: 'j', metaKey: true }) // Mac cmd
    fireEvent.keyDown(list, { key: 'j', isComposing: true })
    expect(active()).toBe('none')
  })

  it('does not open the row when Enter lands on a button inside it', () => {
    const onOpen = vi.fn()
    render(<List items={rows} onOpen={onOpen} />)
    fireEvent.keyDown(screen.getByTestId('list'), { key: 'j' })
    fireEvent.keyDown(screen.getByText('act-a'), { key: 'Enter' })
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('keeps exactly one row tabbable and moves selection when the active row is removed', () => {
    const { rerender } = render(<List items={rows} onOpen={vi.fn()} />)
    const list = screen.getByTestId('list')
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'j' })
    expect(active()).toBe('b')
    expect(rows.filter((r) => screen.getByTestId(`row-${r.id}`).tabIndex === 0)).toHaveLength(1)
    rerender(<List items={[{ id: 'a' }, { id: 'c' }]} onOpen={vi.fn()} />)
    expect(active()).toBe('c')
    rerender(<List items={[]} onOpen={vi.fn()} />)
    expect(active()).toBe('none')
  })
})
