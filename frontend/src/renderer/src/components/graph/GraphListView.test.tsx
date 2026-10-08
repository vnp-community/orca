// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const useVirtualizer = vi.fn()
const scrollToIndex = vi.fn()
vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: (opts: { count: number }) => {
    useVirtualizer(opts)
    const items = Array.from({ length: Math.min(opts.count, 5) }, (_, index) => ({
      index,
      start: index * 40,
      key: index
    }))
    return { getTotalSize: () => opts.count * 40, getVirtualItems: () => items, scrollToIndex }
  }
}))

import { GraphListView, buildGraphListRows } from './GraphListView'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

afterEach(() => {
  cleanup()
  useVirtualizer.mockClear()
})

describe('GraphListView', () => {
  it('sorts by risk descending then label, with relation counts and change text', () => {
    const payload = gPayload(
      [
        gNode('a', { label: 'B-low', risk: 'low' }),
        gNode('b', { label: 'A-crit', risk: 'critical' }),
        gNode('c', { label: 'unk', risk: 'unknown' })
      ],
      [gEdge('a', 'b', { change: 'added' })]
    )
    const rows = buildGraphListRows(payload)
    expect(rows.map((r) => r.node.id)).toEqual(['b', 'a', 'c'])
    expect(rows[0]).toMatchObject({ relations: 1, change: 'added' })
    render(
      <GraphListView payload={payload} selectedId={null} onSelect={vi.fn()} onOpen={vi.fn()} />
    )
    expect(screen.getAllByText('Added').length).toBe(2)
  })

  it('does not virtualize at 100 rows but does above', () => {
    const mk = (n: number) => gPayload(Array.from({ length: n }, (_, i) => gNode(`n${i}`)))
    render(
      <GraphListView payload={mk(100)} selectedId={null} onSelect={vi.fn()} onOpen={vi.fn()} />
    )
    expect(useVirtualizer.mock.calls.every(([o]) => o.count === 0)).toBe(true)
    expect(screen.getAllByRole('row')).toHaveLength(101)
    cleanup()
    useVirtualizer.mockClear()
    render(
      <GraphListView payload={mk(150)} selectedId={null} onSelect={vi.fn()} onOpen={vi.fn()} />
    )
    expect(useVirtualizer.mock.calls.some(([o]) => o.count === 150)).toBe(true)
    expect(screen.getAllByRole('row').length).toBeLessThan(20)
  })

  it('selects with arrows and opens with Enter', () => {
    const onSelect = vi.fn()
    const onOpen = vi.fn()
    render(
      <GraphListView
        payload={gPayload([gNode('a', { risk: 'high' }), gNode('b')])}
        selectedId="a"
        onSelect={onSelect}
        onOpen={onOpen}
      />
    )
    const row = document.querySelector('[data-graph-row-id="a"]') as HTMLElement
    fireEvent.keyDown(row, { key: 'ArrowDown' })
    expect(onSelect).toHaveBeenCalledWith('b')
    fireEvent.keyDown(row, { key: 'Enter' })
    expect(onOpen).toHaveBeenCalledWith('a')
  })

  it('scrolls a virtualized list to the selected row (search pick far down the list)', () => {
    const nodes = Array.from({ length: 150 }, (_, i) => gNode(`n${String(i).padStart(3, '0')}`))
    const { rerender } = render(
      <GraphListView
        payload={gPayload(nodes)}
        selectedId={null}
        onSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(scrollToIndex).not.toHaveBeenCalled()
    rerender(
      <GraphListView
        payload={gPayload(nodes)}
        selectedId="n120"
        onSelect={vi.fn()}
        onOpen={vi.fn()}
      />
    )
    expect(scrollToIndex).toHaveBeenCalledWith(120, { align: 'auto' })
  })

  it('shows "N of total" only when truncated', () => {
    const p = { ...gPayload([gNode('a')]), truncated: true, totalNodes: 9 }
    render(<GraphListView payload={p} selectedId={null} onSelect={vi.fn()} onOpen={vi.fn()} />)
    expect(screen.getByText('Showing 1 of 9')).toBeInTheDocument()
  })
})
