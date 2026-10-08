// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, renderHook, screen } from '@testing-library/react'
import { create } from 'zustand'
import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'

const h = vi.hoisted(() => ({ store: null as unknown }))
vi.mock('@/store', () => ({
  useAppStore: (sel: (s: unknown) => unknown) =>
    (h.store as (s: (x: unknown) => unknown) => unknown)(sel)
}))
vi.mock('@xyflow/react', () => ({ Handle: () => null, Position: { Left: 'l', Right: 'r' } }))

import { useReviewNodeNoteCounts } from './use-review-node-note-counts'
import { ImpactSymbolNode } from '../impact/ImpactSymbolNode'
import { ErdTableNode } from '../erd/ErdTableNode'

const comment = (id: string, sentAt?: number): DiffComment =>
  ({
    id,
    worktreeId: 'wt',
    filePath: 'f',
    lineNumber: 1,
    body: id,
    createdAt: 1,
    sentAt,
    side: 'modified'
  }) as DiffComment
const graph = (lens: string, nodeKey: string): ReviewNoteAnchor =>
  ({ kind: 'graph-node', lens, nodeKey, filePath: 'f', label: nodeKey }) as ReviewNoteAnchor

const NONE: DiffComment[] = []

function seed(comments: DiffComment[], anchors: Record<string, ReviewNoteAnchor>): void {
  h.store = create(() => ({
    getDiffComments: (id: string | null) => (id === 'wt' ? comments : NONE),
    reviewProgressByWorktree: { wt: { serverState: { notes: { anchors, sentBatches: [] } } } }
  }))
}

afterEach(cleanup)

describe('useReviewNodeNoteCounts', () => {
  it('counts unsent anchored notes per node key from the stored ReviewState', () => {
    seed([comment('a'), comment('b'), comment('sent', 5), comment('plain')], {
      a: graph('impact', 'sym1'),
      b: graph('erd', 'orders'),
      sent: graph('impact', 'sym1')
    })
    const { result } = renderHook(() => useReviewNodeNoteCounts('wt'))
    expect(result.current).toEqual({ sym1: 1, orders: 1 })
  })

  it('is empty without a worktree or a loaded row', () => {
    seed([comment('a')], { a: graph('impact', 'sym1') })
    expect(renderHook(() => useReviewNodeNoteCounts(null)).result.current).toEqual({})
    expect(renderHook(() => useReviewNodeNoteCounts('other')).result.current).toEqual({})
  })
})

describe('note badges on graph nodes', () => {
  const impactData = (noteCount: number) => ({
    kind: 'symbol',
    symbol: { key: 'sym1', name: 'Foo', kind: 'function', filePath: 'a.ts' },
    flags: new Set(),
    selected: false,
    column: 1,
    actions: { onSelect: vi.fn(), onFocus: vi.fn(), onOpenDiff: vi.fn() },
    onExpand: vi.fn(),
    noteCount
  })

  it('shows the count on an impact node and hides it at zero', () => {
    const props = { data: impactData(2) } as unknown as Parameters<typeof ImpactSymbolNode>[0]
    const { unmount } = render(<ImpactSymbolNode {...props} />)
    expect(screen.getByLabelText(/2/)).toBeTruthy()
    unmount()
    const zero = {
      data: { ...impactData(0), kind: 'more', hiddenCount: 3 }
    } as unknown as Parameters<typeof ImpactSymbolNode>[0]
    render(<ImpactSymbolNode {...zero} />)
    expect(screen.queryByLabelText(/unsent/i)).toBeNull()
  })

  it('shows the count in an ERD table header', () => {
    const props = {
      selected: false,
      data: {
        table: { key: 'orders', name: 'orders', schema: 'public', columns: [], tableChange: null },
        expanded: false,
        query: '',
        dimmed: false,
        onSelect: vi.fn(),
        onToggleExpand: vi.fn(),
        noteCount: 3
      }
    } as unknown as Parameters<typeof ErdTableNode>[0]
    render(<ErdTableNode {...props} />)
    const header = screen.getByTestId('erd-table-orders')
    expect(header.querySelector('[aria-label]')?.textContent).toBeDefined()
    expect(screen.getByText('3')).toBeTruthy()
  })
})
