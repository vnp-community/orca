// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { useSyncExternalStore } from 'react'
import type { NodeProps } from '@xyflow/react'
import { makeOverlay } from '../review-test-data'
import type { ReviewLensProps } from '../review-lens-registry'
import { sampleStorageMap } from './storage-map.fixture'

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
  return { store, hook: { current: null as unknown } }
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
    nodes: { id: string; data: unknown; type: string }[]
    onNodeClick: (e: unknown, n: unknown) => void
  }) => (
    <div data-testid="flow">
      {nodes.map((n) => (
        <button key={n.id} data-testid={`n-${n.id}`} onClick={() => onNodeClick(null, n)}>
          {n.id}
        </button>
      ))}
    </div>
  ),
  ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useReactFlow: () => ({ fitView: vi.fn() }),
  Background: () => null,
  Controls: () => null,
  Handle: () => null,
  Position: { Left: 'l', Right: 'r' }
}))
vi.mock('../../../hooks/useCodeIntelStorage', () => ({ useCodeIntelStorage: () => h.hook.current }))

import StorageLens from './StorageLens'
import { StorageSecretNode } from './StorageSecretNode'
import { buildStorageViewModel } from './storage-view-model'

const WT = 'wt'
const setStorageEnv = vi.fn()
const selectStorageNode = vi.fn()
const setErdService = vi.fn()
const setReviewLens = vi.fn()
const reload = vi.fn()

function hook(over: Record<string, unknown> = {}, map = sampleStorageMap()) {
  return {
    status: 'success',
    data: map,
    meta: null,
    error: null,
    truncated: false,
    isStale: false,
    available: true,
    reload,
    ...over
  }
}
function setStore(ui: Record<string, unknown> = {}) {
  h.store.set({
    reviewUiByWorktree: { [WT]: ui },
    setStorageEnv,
    selectStorageNode,
    setErdService,
    setReviewLens
  })
}
function props(files: { path: string; status?: string }[] = []): ReviewLensProps {
  return {
    worktreeId: WT,
    environmentId: null,
    scope: { kind: 'branch', baseRef: 'main', includeUncommitted: true },
    overlay: makeOverlay({ changedFiles: files as never }),
    selectedSymbolKey: null,
    chipFilter: null,
    onSelectSymbol: vi.fn(),
    onOpenDiff: vi.fn(),
    requestSymbolChoice: vi.fn()
  } as ReviewLensProps
}

beforeEach(() => {
  vi.clearAllMocks()
  setStore()
  h.hook.current = hook()
})
afterEach(cleanup)

describe('StorageLens states', () => {
  it('renders the four lanes as nodes', () => {
    render(<StorageLens {...props()} />)
    for (const id of [
      'service:infra-fleet',
      'store:pg',
      'store:my',
      'topic:orca.infra.agent',
      'secret:vault_ssh_role'
    ]) {
      expect(screen.getByTestId(`n-${id}`)).toBeInTheDocument()
    }
    expect(screen.queryByTestId('n-store:vault')).toBeNull()
  })

  it('shows loading, empty, error+retry, and hides the map when unsupported', () => {
    h.hook.current = hook({ status: 'loading', data: null })
    render(<StorageLens {...props()} />)
    expect(screen.getByRole('status')).toBeInTheDocument()
    cleanup()
    h.hook.current = hook({}, sampleStorageMap({ stores: [], bindings: [], topics: [] }))
    render(<StorageLens {...props()} />)
    expect(screen.getByText(/No storage configuration was found/)).toBeInTheDocument()
    cleanup()
    h.hook.current = hook({ status: 'error', data: null, error: { kind: 'tool-failed' } })
    render(<StorageLens {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(reload).toHaveBeenCalled()
    cleanup()
    h.hook.current = hook({
      status: 'error',
      data: null,
      error: { kind: 'unsupported' },
      available: false
    })
    render(<StorageLens {...props()} />)
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
    expect(screen.getByText(/not available for this backend/)).toBeInTheDocument()
  })

  it('switches env through the store and shows backend warnings as plain text', () => {
    render(<StorageLens {...props()} />)
    fireEvent.click(screen.getByRole('radio', { name: 'prod' }))
    expect(setStorageEnv).toHaveBeenCalledWith(WT, 'prod')
    expect(screen.getByText('prod topology unknown')).toBeInTheDocument()
    expect(screen.getByText('2 values were redacted by the backend.')).toBeInTheDocument()
  })

  it('marks changed components and says "not detected" - never "safe" - when none changed', () => {
    render(<StorageLens {...props([{ path: 'deploy/my.yml', status: 'modified' }])} />)
    expect(screen.getByRole('checkbox', { name: 'Only changed and related' })).toBeChecked()
    expect(screen.getByTestId('n-store:my')).toBeInTheDocument()
    expect(screen.getByTestId('n-service:agent-gw')).toBeInTheDocument()
    expect(screen.queryByTestId('n-store:pg')).toBeNull()
    cleanup()
    render(<StorageLens {...props([{ path: 'unrelated.go' }])} />)
    expect(
      screen.getByText(/No change to storage configuration was detected within the indexed scope/)
    ).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/\bsafe\b/i)
  })

  it('tells the user when the backend gave no evidence to attribute changes', () => {
    const bare = sampleStorageMap({
      stores: sampleStorageMap().stores.map((s) => ({ ...s, evidence: [] })),
      bindings: sampleStorageMap().bindings.map((b) => ({ ...b, evidence: [] })),
      topics: sampleStorageMap().topics.map((x) => ({ ...x, evidence: [] }))
    })
    h.hook.current = hook({}, bare)
    render(<StorageLens {...props([{ path: 'a.go' }])} />)
    expect(screen.getByText(/Cannot tell which components changed/)).toBeInTheDocument()
  })

  it('detail: "Open ERD" switches lens and service; "View diff" only for changed files; topic lists pub/sub', () => {
    setStore({ selectedStorageNodeId: 'store:pg' })
    const p = props([{ path: 'deploy/pg.yml', status: 'modified' }])
    render(<StorageLens {...p} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open ERD of infra-fleet' }))
    expect(setErdService).toHaveBeenCalledWith(WT, 'infra-fleet')
    expect(setReviewLens).toHaveBeenCalledWith(WT, 'erd')
    fireEvent.click(screen.getByRole('button', { name: 'View diff' }))
    expect(p.onOpenDiff).toHaveBeenCalledWith('deploy/pg.yml')
    cleanup()
    setStore({ selectedStorageNodeId: 'topic:orca.infra.agent' })
    render(<StorageLens {...props()} />)
    const detail = within(screen.getByRole('complementary'))
    expect(detail.getByRole('button', { name: 'agent-gw' })).toBeInTheDocument()
    expect(detail.getByRole('button', { name: 'notifier' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Open ERD/ })).toBeNull()
  })
})

describe('secret safety', () => {
  const CANARIES = ['SUPERSECRETVALUE', 'p4ss', 'abc123']

  it('never renders secret values, in the DOM, localStorage or console', () => {
    const map = sampleStorageMap()
    // Hostile backend: a value smuggled into fields the contract does not define.
    ;(map.stores[2] as Record<string, unknown>).value = 'SUPERSECRETVALUE'
    ;(map.bindings[2] as Record<string, unknown>).payload = 'SUPERSECRETVALUE'
    map.bindings[2] = { ...map.bindings[2], via: 'vault kv password=SUPERSECRETVALUE' }
    h.hook.current = hook({}, map)
    const spies = (['log', 'warn', 'error', 'info'] as const).map((m) =>
      vi.spyOn(console, m).mockImplementation(() => undefined)
    )
    setStore({ selectedStorageNodeId: 'secret:vault_ssh_role' })
    const { container } = render(<StorageLens {...props()} />)
    fireEvent.click(screen.getByTestId('n-store:pg'))
    setStore({ selectedStorageNodeId: 'store:pg' })
    fireEvent.click(screen.getByText('Text view'))
    const html = container.innerHTML + document.body.textContent
    for (const c of CANARIES) {
      expect(html).not.toContain(c)
    }
    expect(JSON.stringify({ ...localStorage })).not.toContain('SUPERSECRET')
    for (const spy of spies) {
      expect(JSON.stringify(spy.mock.calls)).not.toMatch(/SUPERSECRET|p4ss|abc123/)
      spy.mockRestore()
    }
  })

  it('secret detail offers only key-name copy, no reveal control', () => {
    setStore({ selectedStorageNodeId: 'secret:vault_ssh_role' })
    render(<StorageLens {...props()} />)
    expect(screen.getByText('Vault path: secret/data/infra/ssh')).toBeInTheDocument()
    expect(screen.getByText('The value is never shown or copied.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /reveal|show value|copy value/i })).toBeNull()
  })
})

describe('StorageSecretNode', () => {
  it('reads only name and vault path from the node', () => {
    const vm = buildStorageViewModel(sampleStorageMap())
    const secret = {
      ...vm.nodes.find((n) => n.lane === 'secret')!,
      payload: 'LEAKED',
      via: 'LEAKED'
    } as never
    render(
      <StorageSecretNode
        {...({ data: { node: secret, dimmed: false }, selected: false } as unknown as NodeProps)}
      />
    )
    expect(screen.getByText('vault_ssh_role')).toBeInTheDocument()
    expect(screen.getByText('secret/data/infra/ssh')).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('LEAKED')
  })

  it('shows the masked-text icon when the key name itself needed masking', () => {
    const node = {
      id: 'secret:x',
      lane: 'secret',
      name: '•••',
      masked: true,
      evidencePaths: []
    } as never
    render(
      <StorageSecretNode
        {...({ data: { node, dimmed: false }, selected: false } as unknown as NodeProps)}
      />
    )
    expect(screen.getByLabelText(/masked in the interface/)).toBeInTheDocument()
  })
})
