// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useSyncExternalStore } from 'react'
import type { DataFlow, DataFlowSummary, SequenceModel } from '../../../../../shared/code-intel-architecture-types'
import { makeOverlay } from '../review-test-data'
import type { ReviewLensProps } from '../review-lens-registry'

const h = vi.hoisted(() => {
  const listeners = new Set<() => void>()
  const store = {
    state: {} as Record<string, unknown>,
    set(next: Record<string, unknown>) { store.state = next; listeners.forEach((l) => l()) },
    subscribe(l: () => void) { listeners.add(l); return () => listeners.delete(l) }
  }
  return { store, list: { current: null as unknown }, flow: { current: null as unknown }, mermaid: vi.fn() }
})

vi.mock('@/store', () => ({
  useAppStore: (sel: (s: Record<string, unknown>) => unknown) => useSyncExternalStore(h.store.subscribe, () => sel(h.store.state))
}))
vi.mock('../../../hooks/useDataFlows', () => ({ useDataFlows: () => h.list.current }))
vi.mock('../../../hooks/useDataFlow', () => ({ useDataFlow: () => h.flow.current }))
vi.mock('../../../hooks/useIsDarkTheme', () => ({ useIsDarkTheme: () => false }))
vi.mock('../../editor/MermaidBlock', () => ({
  default: (p: { content: string }) => { h.mermaid(p); return <svg data-testid="mermaid"><text>{p.content.length}</text></svg> }
}))

import DataFlowLens from './DataFlowLens'

const summary = (id: string, over: Partial<DataFlowSummary> = {}): DataFlowSummary => ({
  id, label: `Flow ${id}`, trigger: { kind: 'grpc', name: `rpc.${id}` }, entryService: 's', entryRpc: 'r', serviceHops: 2, completeness: 'complete', ...over
})
const ref = (name: string) => ({ container: 'c', componentId: name, name, kind: 'component' as const })
const symbol = { key: 'sym1', kind: 'func', name: 'Create', filePath: 'a.go' } as never
const flowData = (over: Partial<DataFlow> = {}): DataFlow => ({
  id: 'f1', label: 'Flow f1', trigger: { kind: 'grpc', name: 'rpc.f1' },
  steps: [
    { n: 1, from: ref('UI'), to: ref('Order'), kind: 'rpc', sync: true, confidence: 1, origin: 'declared', evidence: [], symbol },
    { n: 2, from: ref('Order'), to: ref('DB'), kind: 'db-write', sync: true, confidence: 0.6, origin: 'static-name', evidence: [] }
  ],
  stores: [], completeness: 'complete', gaps: [], services: [], relatedProcesses: [], ...over
})
const seq: SequenceModel = {
  participants: [{ id: 'ui', label: 'UI', kind: 'ui' }, { id: 'o', label: 'Order', kind: 'component' }, { id: 'db', label: 'DB', kind: 'external' }],
  messages: [
    { n: 1, from: 'ui', to: 'o', label: 'create', kind: 'rpc', sync: true, dashedReturn: false, confidence: 1 },
    { n: 2, from: 'o', to: 'db', label: 'insert', kind: 'db-write', sync: true, dashedReturn: false, confidence: 1 }
  ]
}
const listState = (flows: DataFlowSummary[], over = {}) => ({
  status: 'ready', flows, total: flows.length, hasMore: false, loadingMore: false, error: null, loadMore: vi.fn(), refetch: vi.fn(), ...over
})
const flowState = (over = {}) => ({ status: 'ready', data: { flow: flowData(), sequence: seq }, meta: null, error: null, refetch: vi.fn(), ...over })

const onSelectSymbol = vi.fn()
const props = (over: Partial<ReviewLensProps> = {}): ReviewLensProps => ({
  worktreeId: 'w', environmentId: null,
  scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
  overlay: makeOverlay(), selectedSymbolKey: null, chipFilter: null,
  onSelectSymbol, onOpenDiff: vi.fn(), requestSymbolChoice: vi.fn(), ...over
})
const setFlowId = vi.fn((wt: string, id: string) => h.store.set({ ...h.store.state, reviewUiByWorktree: { [wt]: { dataFlowId: id } } }))

beforeEach(() => {
  vi.clearAllMocks()
  h.store.state = { reviewUiByWorktree: {}, setReviewDataFlowId: setFlowId }
  h.list.current = listState([summary('f1'), summary('f2', { completeness: 'partial' })])
  h.flow.current = flowState()
})
afterEach(cleanup)

describe('DataFlowLens', () => {
  it('lists flows with a partial tag and prompts to pick one', () => {
    render(<DataFlowLens {...props()} />)
    expect(screen.getByText('Flow f1')).toBeInTheDocument()
    expect(screen.getByText('partial')).toBeInTheDocument()
    expect(screen.getByText('Pick a flow to see its sequence.')).toBeInTheDocument()
  })

  it('selecting a flow stores the id and draws the diagram via MermaidBlock', () => {
    render(<DataFlowLens {...props()} />)
    fireEvent.click(screen.getByText('Flow f1'))
    expect(setFlowId).toHaveBeenCalledWith('w', 'f1')
    expect(screen.getByTestId('mermaid')).toBeInTheDocument()
    const call = h.mermaid.mock.calls.at(-1)![0]
    expect(call.content).toContain('sequenceDiagram')
    expect(call.htmlLabels).toBe(false)
    expect(call.isDark).toBe(false)
  })

  it('shows the partial banner, gap rows and the Inferred label', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    h.flow.current = flowState({ data: { flow: flowData({ completeness: 'partial', gaps: [{ afterStep: 1, code: 'x', message: 'callee unknown' }] }), sequence: seq } })
    render(<DataFlowLens {...props()} />)
    expect(screen.getByText(/This flow is incomplete/)).toBeInTheDocument()
    expect(screen.getByText('Data is missing after step 1')).toBeInTheDocument()
    expect(screen.getByText('Inferred')).toBeInTheDocument()
  })

  it('marks changed steps with text and a data attribute, and prefixes the diagram', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    const overlay = makeOverlay({ changedSymbols: [{ symbol: symbol as never, changeKind: 'modified', tested: 'no' }] })
    render(<DataFlowLens {...props({ overlay })} />)
    expect(screen.getByText(/● changed/)).toBeInTheDocument()
    expect(document.querySelector('[data-changed="true"]')).not.toBeNull()
    expect(h.mermaid.mock.calls.at(-1)![0].content).toContain('[changed] create')
  })

  it('activating a step with a symbol opens its detail; without one shows step detail', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    render(<DataFlowLens {...props()} />)
    const options = screen.getAllByRole('option')
    fireEvent.click(options[0])
    expect(onSelectSymbol).toHaveBeenCalledWith('sym1')
    fireEvent.click(options[1])
    expect(screen.getByLabelText('Step details')).toBeInTheDocument()
  })

  it('supports keyboard navigation in the step list', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    render(<DataFlowLens {...props()} />)
    const list = screen.getByRole('listbox')
    fireEvent.keyDown(list, { key: 'j' })
    expect(list).toHaveAttribute('aria-activedescendant', 'dataflow-row-1')
    fireEvent.keyDown(list, { key: 'Enter' })
    expect(screen.getByLabelText('Step details')).toBeInTheDocument()
  })

  it('copies the Mermaid source and shows Copied', async () => {
    const write = vi.fn().mockResolvedValue(undefined)
    ;(window as unknown as { api: unknown }).api = { ui: { writeClipboardText: write } }
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    render(<DataFlowLens {...props()} />)
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Copy Mermaid' })) })
    expect(write).toHaveBeenCalledWith(expect.stringContaining('sequenceDiagram'))
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('renders an unknown flow id as a message, not an error screen', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'ghost' } } }
    h.flow.current = { status: 'error', data: null, meta: null, error: { kind: 'not-found', message: 'x' }, refetch: vi.fn() }
    render(<DataFlowLens {...props()} />)
    expect(screen.getByRole('alert')).toHaveTextContent('No data flow matches')
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })

  it('touch-only toggle says "cannot tell" when nothing intersects, and filters by id otherwise', () => {
    render(<DataFlowLens {...props()} />)
    fireEvent.click(screen.getByLabelText('Only flows touching changes'))
    expect(screen.getByText('Cannot tell yet which flows touch the changes.')).toBeInTheDocument()
    expect(screen.getByText('Flow f2')).toBeInTheDocument()
    cleanup()
    render(<DataFlowLens {...props({ chipFilter: 'flows', overlay: makeOverlay({ affectedFlows: [{ id: 'f2' }] }) })} />)
    expect(screen.queryByText('Flow f1')).toBeNull()
    expect(screen.getByText('Flow f2')).toBeInTheDocument()
  })

  it('shows empty and error states of the list with retry', () => {
    const refetch = vi.fn()
    h.list.current = listState([], { status: 'error', error: { message: 'boom' }, refetch })
    render(<DataFlowLens {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalled()
    cleanup()
    h.list.current = listState([])
    render(<DataFlowLens {...props()} />)
    expect(screen.getByText(/No data flows could be built/)).toBeInTheDocument()
  })

  it('too many participants: no diagram, list still full', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    const ids = Array.from({ length: 16 }, (_, i) => `x${i}`)
    h.flow.current = flowState({
      data: {
        flow: flowData(),
        sequence: { participants: [], messages: ids.slice(1).map((id, i) => ({ n: i + 1, from: ids[i], to: id, label: 'm', kind: 'rpc', sync: true, dashedReturn: false, confidence: 1 })) }
      }
    })
    render(<DataFlowLens {...props()} />)
    expect(screen.queryByTestId('mermaid')).toBeNull()
    expect(screen.getByText(/Too many participants/)).toBeInTheDocument()
    expect(screen.getAllByRole('option')).toHaveLength(2)
  })

  it('virtualizes long step lists without crashing', () => {
    h.store.state = { ...h.store.state, reviewUiByWorktree: { w: { dataFlowId: 'f1' } } }
    const steps = Array.from({ length: 200 }, (_, i) => ({ n: i + 1, from: ref('A'), to: ref('B'), kind: 'call' as const, sync: true, confidence: 1, origin: 'declared' as const, evidence: [] }))
    h.flow.current = flowState({ data: { flow: flowData({ steps }), sequence: seq } })
    render(<DataFlowLens {...props()} />)
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    expect(screen.queryAllByRole('option').length).toBeLessThan(200)
  })
})
