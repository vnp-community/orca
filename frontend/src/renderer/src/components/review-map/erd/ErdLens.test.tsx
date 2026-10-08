// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useSyncExternalStore } from 'react'
import { makeOverlay } from '../review-test-data'
import type { ReviewLensProps } from '../review-lens-registry'
import { sampleErdModel } from './erd-model.fixture'

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
  return { store, erd: { current: null as unknown } }
})

vi.mock('@/store', () => ({
  useAppStore: (sel: (s: Record<string, unknown>) => unknown) =>
    useSyncExternalStore(h.store.subscribe, () => sel(h.store.state))
}))
vi.mock('@xyflow/react', () => ({
  ReactFlow: ({
    nodes,
    onNodeClick
  }: {
    nodes: { id: string; type: string }[]
    onNodeClick: (e: unknown, n: unknown) => void
  }) => (
    <div data-testid="flow">
      {nodes
        .filter((n) => n.type !== 'erdGroup')
        .map((n) => (
          <button key={n.id} data-testid={`n-${n.id}`} onClick={() => onNodeClick(null, n)}>
            {n.id}
          </button>
        ))}
    </div>
  ),
  ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useReactFlow: () => ({ setCenter: vi.fn(), getZoom: () => 1 }),
  Background: () => null,
  Controls: () => null,
  MiniMap: () => null,
  Handle: () => null,
  Position: { Left: 'l', Right: 'r' }
}))
// The note button is store-bound and covered by notes/; ErdTableDetail tests check its anchor.
vi.mock('../notes/ReviewNoteButton', () => ({ ReviewNoteButton: () => null }))
vi.mock('../notes/use-review-node-note-counts', () => ({ useReviewNodeNoteCounts: () => ({}) }))
vi.mock('../../../hooks/useCodeIntelErd', () => ({ useCodeIntelErd: () => h.erd.current }))

import ErdLens from './ErdLens'

const WT = 'wt'
const setErdService = vi.fn()
const goBackErdService = vi.fn()
const selectErdTable = vi.fn()
const reload = vi.fn()

function ready(over: Record<string, unknown> = {}, model = sampleErdModel()) {
  return {
    services: {
      status: 'success',
      data: [{ name: 'infra-fleet', dialects: ['postgres'], tableCount: 3 }],
      error: null
    },
    model: { status: 'success', data: model, meta: null, error: null, truncated: false },
    resolved: { service: 'infra-fleet', dialect: 'postgres' },
    isStale: false,
    reload,
    ...over
  }
}
function setStore(ui: Record<string, unknown> = {}) {
  h.store.set({ reviewUiByWorktree: { [WT]: ui }, setErdService, goBackErdService, selectErdTable })
}
function props(over: Partial<ReviewLensProps> = {}): ReviewLensProps {
  return {
    worktreeId: WT,
    environmentId: null,
    scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
    overlay: makeOverlay(),
    selectedSymbolKey: null,
    chipFilter: null,
    onSelectSymbol: vi.fn(),
    onOpenDiff: vi.fn(),
    requestSymbolChoice: vi.fn(),
    ...over
  } as ReviewLensProps
}

beforeEach(() => {
  vi.clearAllMocks()
  setStore()
  h.erd.current = ready()
})
afterEach(cleanup)

describe('ErdLens', () => {
  it('renders tables of the resolved service and selects one from the canvas', () => {
    render(<ErdLens {...props()} />)
    expect(screen.getByTestId('n-infra.dev_servers')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('n-infra.agents'))
    expect(selectErdTable).toHaveBeenCalledWith(WT, 'infra.agents')
  })

  it('draws a ghost node for a cross-service table', () => {
    render(<ErdLens {...props()} />)
    expect(screen.getByTestId('n-ghost:auth.tenants')).toBeInTheDocument()
  })

  it('shows the loading placeholder while the model loads', () => {
    h.erd.current = ready({
      model: { status: 'loading', data: null, meta: null, error: null, truncated: false }
    })
    render(<ErdLens {...props()} />)
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.queryByTestId('flow')).toBeNull()
  })

  it('shows an inline error with Retry for the model phase and the services phase', () => {
    h.erd.current = ready({
      model: {
        status: 'error',
        data: null,
        meta: null,
        error: { kind: 'too-large' },
        truncated: false
      }
    })
    render(<ErdLens {...props()} />)
    expect(screen.getByRole('alert')).toHaveTextContent('too large')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(reload).toHaveBeenCalled()
    cleanup()
    h.erd.current = ready({ services: { status: 'error', data: null, error: { kind: 'offline' } } })
    render(<ErdLens {...props()} />)
    expect(screen.getByRole('alert')).toHaveTextContent('connection')
  })

  it('shows an empty state (not an error) when no service has migrations', () => {
    h.erd.current = ready({
      services: { status: 'success', data: [], error: null },
      resolved: { service: null, dialect: null },
      model: { status: 'idle', data: null, meta: null, error: null, truncated: false }
    })
    render(<ErdLens {...props()} />)
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByText(/No services with migrations/)).toBeInTheDocument()
  })

  it('reports an empty service inline', () => {
    h.erd.current = ready(
      {},
      sampleErdModel({ tables: [], relations: [], changes: [], externalRefs: [] })
    )
    render(<ErdLens {...props()} />)
    expect(screen.getByText(/no tables in its migrations/)).toBeInTheDocument()
  })

  it('shows warnings, the backend-truncated notice and the stale chip; the chip reloads', () => {
    const model = sampleErdModel({ warnings: [{ file: 'm.sql', code: 'W', message: '<b>x</b>' }] })
    h.erd.current = ready(
      {
        isStale: true,
        model: { status: 'success', data: model, meta: null, error: null, truncated: true }
      },
      model
    )
    render(<ErdLens {...props()} />)
    expect(screen.getByTestId('erd-warnings')).toBeInTheDocument()
    expect(screen.getByText('The backend limited this result.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /New data available/ }))
    expect(reload).toHaveBeenCalled()
  })

  it('offers "Clear filters" when focus mode has nothing to show', () => {
    h.erd.current = ready({}, sampleErdModel({ changes: [] }))
    render(<ErdLens {...props()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Changed + related' }))
    expect(screen.getByText('No tables match the current filters.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Clear filters/ }))
    expect(screen.getByTestId('n-infra.agents')).toBeInTheDocument()
  })

  it('switches to the list with "l", back with "g", and clears selection with Escape', () => {
    setStore({ selectedErdTable: 'infra.agents' })
    render(<ErdLens {...props()} />)
    const root = screen.getByRole('region', { name: 'Entity-relationship diagram' })
    fireEvent.keyDown(root, { key: 'l' })
    expect(screen.getByTestId('erd-list-infra.agents')).toBeInTheDocument()
    fireEvent.keyDown(root, { key: 'g' })
    expect(screen.getByTestId('flow')).toBeInTheDocument()
    fireEvent.keyDown(root, { key: 'Escape' })
    expect(selectErdTable).toHaveBeenCalledWith(WT, null)
  })

  it('ignores shortcuts typed into the search box', () => {
    render(<ErdLens {...props()} />)
    fireEvent.keyDown(screen.getByLabelText('Search the ERD'), { key: 'l' })
    expect(screen.queryByTestId('erd-list-infra.agents')).toBeNull()
  })

  it('shows the selected table detail with the back button after a service hop', () => {
    setStore({ selectedErdTable: 'infra.dev_servers', erdServiceHistory: ['auth'] })
    render(<ErdLens {...props()} />)
    expect(screen.getByRole('complementary')).toHaveAccessibleName(/infra.dev_servers/)
    fireEvent.click(screen.getByRole('button', { name: 'Back to previous service' }))
    expect(goBackErdService).toHaveBeenCalledWith(WT)
  })

  it('caps the rendered tables and says so', () => {
    const tables = Array.from({ length: 160 }, (_, i) => ({
      ...sampleErdModel().tables[2],
      name: `t${i}`
    }))
    const model = sampleErdModel({ tables, relations: [], changes: [], externalRefs: [] })
    h.erd.current = ready({}, model)
    render(<ErdLens {...props()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'All' }))
    expect(
      screen.getByText('Showing 150 of 160 tables; the rest are in the list.')
    ).toBeInTheDocument()
  })
})
