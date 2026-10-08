// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Edge, Node } from '@xyflow/react'

let lastNodes: Node[] = []
let lastEdges: Edge[] = []
let hasMiniMap = false
let fitView = vi.fn()
let emitMove: ((zoom: number) => void) | null = null

vi.mock('@xyflow/react', () => ({
  ReactFlow: ({ nodes, edges, children, onInit, onNodeClick, onMove }: { onMove?: (e: unknown, v: { zoom: number }) => void; nodes: Node[]; edges: Edge[]; children: React.ReactNode; onInit?: (i: unknown) => void; onNodeClick?: (e: unknown, n: Node) => void }) => {
    lastNodes = nodes
    lastEdges = edges
    onInit?.({ fitView })
    emitMove = (zoom) => onMove?.(null, { zoom })
    return (
      <div data-testid="flow">
        {nodes.map((n) => (
          <button key={n.id} data-testid={`n-${n.id}`} onClick={() => onNodeClick?.(null, n)}>{n.id}</button>
        ))}
        {children}
      </div>
    )
  },
  Background: () => null,
  Controls: () => null,
  MiniMap: () => { hasMiniMap = true; return null },
  Handle: () => null,
  Position: { Left: 'left', Right: 'right' },
  BaseEdge: () => null,
  EdgeLabelRenderer: () => null,
  getBezierPath: () => ['', 0, 0]
}))
vi.mock('@xyflow/react/dist/style.css', () => ({}))

import { GraphCanvas, type GraphCanvasProps } from './GraphCanvas'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

afterEach(() => { cleanup(); hasMiniMap = false; fitView = vi.fn() })

function props(over: Partial<GraphCanvasProps> = {}): GraphCanvasProps {
  return {
    payload: gPayload([gNode('a'), gNode('b')], [gEdge('a', 'b')]),
    selectedId: null, onSelect: vi.fn(), onOpenNode: vi.fn(), openGroups: new Set(), onToggleGroup: vi.fn(),
    changeView: 'after', focusId: null, onFocus: vi.fn(), ...over
  }
}

const big = (n: number) => gPayload(Array.from({ length: n }, (_, i) => gNode(`n${String(i).padStart(3, '0')}`, { group: `g${i % 4}` })))

describe('GraphCanvas semantic zoom', () => {
  it('applies zoom level to node detail and edge presentation', async () => {
    vi.useFakeTimers()
    try {
      render(<GraphCanvas {...props()} />)
      expect(screen.getByTestId('graph-canvas')).toHaveAttribute('data-zoom-level', 'module')
      expect((lastNodes[0].data as { detail: string }).detail).toBe('compact')
      act(() => { emitMove?.(0.3); vi.advanceTimersByTime(150) })
      expect(screen.getByTestId('graph-canvas')).toHaveAttribute('data-zoom-level', 'service')
      expect((lastNodes[0].data as { detail: string }).detail).toBe('minimal')
      expect((lastEdges[0].data as { showSign: boolean }).showSign).toBe(false)
      act(() => { emitMove?.(2); vi.advanceTimersByTime(150) })
      expect((lastNodes[0].data as { detail: string }).detail).toBe('full')
      expect((lastEdges[0].data as { showSign: boolean }).showSign).toBe(true)
    } finally { vi.useRealTimers() }
  })
})

describe('GraphCanvas', () => {
  it('caps visible nodes at 50 plus group nodes, and shows the minimap above 30 nodes', async () => {
    render(<GraphCanvas {...props({ payload: big(120) })} />)
    await waitFor(() => expect(lastNodes.length).toBeGreaterThan(50))
    expect(lastNodes.filter((n) => n.type === 'graphNode')).toHaveLength(50)
    expect(lastNodes.filter((n) => n.type === 'graphGroup').length).toBeGreaterThan(0)
    expect(hasMiniMap).toBe(true)
  })

  it('does not show the minimap for small graphs', async () => {
    render(<GraphCanvas {...props()} />)
    await waitFor(() => expect(lastNodes).toHaveLength(2))
    expect(hasMiniMap).toBe(false)
  })

  it('changes the edge set between before and after views', async () => {
    const payload = gPayload([gNode('a'), gNode('b'), gNode('c')], [gEdge('a', 'b', { change: 'removed' }), gEdge('a', 'c', { change: 'added' })])
    const { rerender } = render(<GraphCanvas {...props({ payload, changeView: 'after' })} />)
    await waitFor(() => expect(lastEdges).toHaveLength(2))
    rerender(<GraphCanvas {...props({ payload, changeView: 'before' })} />)
    await waitFor(() => expect(lastEdges).toHaveLength(1))
  })

  it('Escape exits focus and node click selects', async () => {
    const p = props({ focusId: 'a' })
    render(<GraphCanvas {...p} />)
    fireEvent.keyDown(screen.getByTestId('graph-canvas'), { key: 'Escape' })
    expect(p.onFocus).toHaveBeenCalledWith(null)
    fireEvent.click(screen.getByTestId('n-a'))
    expect(p.onSelect).toHaveBeenCalledWith('a')
  })

  it('dims nodes outside the focus neighborhood', async () => {
    const payload = gPayload([gNode('a'), gNode('b'), gNode('z')], [gEdge('a', 'b')])
    render(<GraphCanvas {...props({ payload, focusId: 'a' })} />)
    await waitFor(() => expect(lastNodes).toHaveLength(3))
    const dimmed = Object.fromEntries(lastNodes.map((n) => [n.id, (n.data as { dimmed: boolean }).dimmed]))
    expect(dimmed).toEqual({ a: false, b: false, z: true })
  })

  it('opening a group puts its members into the visible set', async () => {
    const payload = big(120)
    const closed = render(<GraphCanvas {...props({ payload })} />)
    await waitFor(() => expect(lastNodes.length).toBeGreaterThan(50))
    const before = lastNodes.filter((n) => n.type === 'graphNode').length
    closed.unmount()
    render(<GraphCanvas {...props({ payload, openGroups: new Set(['g0']) })} />)
    await waitFor(() => expect(lastNodes.filter((n) => n.type === 'graphNode').length).toBeGreaterThan(before))
  })

  it('fits the view when the layout is ready', async () => {
    render(<GraphCanvas {...props()} />)
    await act(async () => {})
    await waitFor(() => expect(fitView).toHaveBeenCalled())
  })
})
