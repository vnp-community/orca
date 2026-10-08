// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useSyncExternalStore } from 'react'
import type { C4ComponentView, ContainerRef } from '../../../../../shared/code-intel-architecture-types'
import { makeOverlay } from '../review-test-data'
import type { ReviewLensProps } from '../review-lens-registry'

const h = vi.hoisted(() => {
  const listeners = new Set<() => void>()
  const store = {
    state: {} as Record<string, unknown>,
    set(next: Record<string, unknown>) {
      store.state = next
      listeners.forEach((l) => l())
    },
    subscribe(l: () => void) {
      listeners.add(l)
      return () => listeners.delete(l)
    }
  }
  return { store, load: { current: null as unknown } }
})

vi.mock('@/store', () => ({
  useAppStore: (sel: (s: Record<string, unknown>) => unknown) =>
    useSyncExternalStore(h.store.subscribe, () => sel(h.store.state))
}))
vi.mock('@xyflow/react', () => ({
  ReactFlow: ({ nodes, edges, onNodeClick }: { nodes: { id: string; type: string }[]; edges: unknown[]; onNodeClick: (e: unknown, n: unknown) => void }) => (
    <div data-testid="flow" data-edges={edges.length}>
      {nodes.filter((n) => n.type !== 'c4Band').map((n) => (
        <button key={n.id} data-testid={`n-${n.id}`} onClick={() => onNodeClick(null, n)}>{n.id}</button>
      ))}
    </div>
  ),
  Background: () => null,
  Controls: () => null
}))
vi.mock('../../../hooks/useC4Architecture', () => ({ useC4Architecture: () => h.load.current }))
vi.mock('./C4OverrideEditor', () => ({ C4OverrideEditor: () => <div data-testid="editor" /> }))

import ArchitectureLens from './ArchitectureLens'

const containers: ContainerRef[] = [
  { id: 'a', name: 'alpha', path: 'svc/alpha', kind: 'service' },
  { id: 'b', name: 'beta', path: 'svc/beta', kind: 'service' }
]
const view = (over: Partial<C4ComponentView> = {}): C4ComponentView => ({
  container: containers[1],
  components: [
    { id: 'uc', name: 'usecase', kind: 'usecase', path: 'uc', descriptionSource: 'none', symbolCount: 3, origin: 'derived', packagePaths: ['uc'], hidden: false },
    { id: 'dom', name: 'domain', kind: 'domain', path: 'dom', descriptionSource: 'none', symbolCount: 1, origin: 'merged', packagePaths: ['dom'], hidden: false }
  ],
  relations: [{ from: 'uc', to: 'dom', kind: 'uses', evidence: [], count: 2, origin: 'derived', confidence: 1, violatesLayering: false }],
  externals: [],
  warnings: [],
  overridesVersion: '0',
  hasOverrides: false,
  ...over
})
const ready = (v: C4ComponentView | null) => ({ status: 'ready', data: { containers, view: v }, meta: null, error: null, refetch: vi.fn() })

const props = (over: Partial<ReviewLensProps> = {}): ReviewLensProps => ({
  worktreeId: 'w',
  environmentId: null,
  scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
  overlay: makeOverlay({ changedFiles: [{ path: 'svc/beta/uc/x.go', status: 'modified' }] }),
  selectedSymbolKey: null,
  chipFilter: null,
  onSelectSymbol: vi.fn(),
  onOpenDiff: vi.fn(),
  requestSymbolChoice: vi.fn(),
  ...over
})

beforeEach(() => {
  const setReviewC4Container = vi.fn((wt: string, id: string) =>
    h.store.set({ ...h.store.state, reviewUiByWorktree: { [wt]: { c4ContainerId: id } } })
  )
  h.store.state = { reviewUiByWorktree: {}, setReviewC4Container, setReviewC4Draft: vi.fn() }
  h.load.current = ready(view())
})
afterEach(cleanup)

describe('ArchitectureLens', () => {
  it('picks the container with the most changed files and persists it', () => {
    render(<ArchitectureLens {...props()} />)
    expect((h.store.state.setReviewC4Container as ReturnType<typeof vi.fn>).mock.calls[0]).toEqual(['w', 'b'])
  })

  it('always shows the inferred notice when there are no overrides', () => {
    render(<ArchitectureLens {...props()} />)
    expect(screen.getByRole('note')).toHaveTextContent(/inferred/i)
  })

  it('renders nodes and opens the detail with an origin label on click', () => {
    render(<ArchitectureLens {...props()} />)
    expect(screen.getByTestId('flow')).toHaveAttribute('data-edges', '1')
    fireEvent.click(screen.getByTestId('n-dom'))
    expect(screen.getByRole('complementary')).toHaveTextContent('Partly edited')
    expect(screen.getByRole('complementary')).toHaveTextContent('No description yet')
  })

  it('switches to the list and shows the shown/total counter', () => {
    render(<ArchitectureLens {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: 'List' }))
    expect(screen.getByText('Showing 1/1')).toBeInTheDocument()
  })

  it('defaults to the list above 150 nodes', () => {
    const many = Array.from({ length: 151 }, (_, i) => ({
      id: `c${i}`, name: `c${i}`, kind: 'usecase' as const, path: `c${i}`, descriptionSource: 'none' as const,
      symbolCount: 1, origin: 'derived' as const, packagePaths: [`c${i}`], hidden: false
    }))
    h.load.current = ready(view({ components: many, relations: [] }))
    render(<ArchitectureLens {...props()} />)
    expect(screen.queryByTestId('flow')).toBeNull()
    expect(screen.getByText('Showing 0/0')).toBeInTheDocument()
  })

  it('shows retry on error and empty-container copy', () => {
    const refetch = vi.fn()
    h.load.current = { status: 'error', data: null, meta: null, error: { message: 'boom' }, refetch }
    render(<ArchitectureLens {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalled()
    cleanup()
    h.load.current = { status: 'ready', data: { containers: [], view: null }, meta: null, error: null, refetch }
    render(<ArchitectureLens {...props()} />)
    expect(screen.getByText(/No containers were inferred/)).toBeInTheDocument()
  })

  it('shows the empty-view message when view is null', () => {
    h.load.current = ready(null)
    render(<ArchitectureLens {...props()} />)
    expect(screen.getByText(/No component view/)).toBeInTheDocument()
  })

  it('opens the editor from the toolbar', () => {
    render(<ArchitectureLens {...props()} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit c4.yaml' })[0])
    expect(screen.getByTestId('editor')).toBeInTheDocument()
  })
})
