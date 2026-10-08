// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useSyncExternalStore } from 'react'
import type { DataFlowSummary } from '../../../../../shared/code-intel-architecture-types'
import type { ReviewLensProps } from '../review-lens-registry'
import { makeOverlay } from '../review-test-data'

const h = vi.hoisted(() => ({
  filters: [] as unknown[],
  list: null as unknown,
  state: {} as Record<string, unknown>,
  setFlowId: vi.fn(),
  resize: null as null | ((width: number) => void)
}))

vi.mock('@/store', () => ({
  useAppStore: (sel: (s: Record<string, unknown>) => unknown) =>
    useSyncExternalStore(
      () => () => undefined,
      () => sel(h.state)
    )
}))
vi.mock('../../../hooks/useDataFlows', () => ({
  useDataFlows: (_w: string, _e: string | null, filter: unknown) => {
    h.filters.push(filter)
    return h.list
  }
}))
vi.mock('./DataFlowDetailPane', () => ({ DataFlowDetailPane: () => null }))
// Native stand-in for the Radix Select so options can be chosen in happy-dom.
vi.mock('@/components/ui/select', async () => {
  const React = await import('react')
  const Ctx = React.createContext<{
    value?: string
    onValueChange?: (v: string) => void
    label?: string
  }>({})
  return {
    Select: ({
      value,
      onValueChange,
      children
    }: {
      value?: string
      onValueChange?: (v: string) => void
      children: React.ReactNode
    }) => {
      const ctx = React.useMemo(() => ({ value, onValueChange }), [value, onValueChange])
      return <Ctx.Provider value={ctx}>{children}</Ctx.Provider>
    },
    SelectTrigger: ({ 'aria-label': label }: { 'aria-label'?: string }) => {
      const ctx = React.useContext(Ctx)
      return <span data-testid={`trigger-${label}`} data-value={ctx.value ?? ''} />
    },
    SelectValue: () => null,
    SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => {
      const ctx = React.useContext(Ctx)
      return (
        <button
          type="button"
          role="option"
          aria-selected={ctx.value === value}
          onClick={() => ctx.onValueChange?.(value)}
        >
          {children}
        </button>
      )
    }
  }
})

import DataFlowLens from './DataFlowLens'

const summary = (
  id: string,
  entryService: string,
  over: Partial<DataFlowSummary> = {}
): DataFlowSummary => ({
  id,
  label: `Flow ${id}`,
  trigger: { kind: 'grpc', name: `rpc.${id}` },
  entryService,
  entryRpc: 'r',
  serviceHops: 1,
  completeness: 'complete',
  ...over
})

const props = {
  worktreeId: 'w',
  environmentId: null,
  overlay: makeOverlay(),
  chipFilter: null,
  onSelectSymbol: vi.fn()
} as unknown as ReviewLensProps

beforeEach(() => {
  h.filters.length = 0
  h.state = { reviewUiByWorktree: {}, setReviewDataFlowId: h.setFlowId }
  h.list = {
    status: 'ready',
    flows: [summary('a', 'orders'), summary('b', 'billing', { completeness: 'partial' })],
    total: 2,
    hasMore: false,
    loadingMore: false,
    error: null,
    loadMore: vi.fn(),
    refetch: vi.fn()
  }
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(cb: (entries: { contentRect: { width: number } }[]) => void) {
        h.resize = (width) => cb([{ contentRect: { width } }])
      }
      observe(): void {}
      disconnect(): void {}
    }
  )
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('DataFlowLens filters', () => {
  it('sends triggerKind and service to the server query and can clear them', () => {
    render(<DataFlowLens {...props} />)
    expect(h.filters.at(-1)).toEqual({ query: '', triggerKind: null, service: null })
    fireEvent.click(screen.getByRole('option', { name: 'HTTP' }))
    expect(h.filters.at(-1)).toMatchObject({ triggerKind: 'http', service: null })
    fireEvent.click(screen.getByRole('option', { name: 'billing' }))
    expect(h.filters.at(-1)).toMatchObject({ triggerKind: 'http', service: 'billing' })
    fireEvent.click(screen.getByRole('option', { name: 'All triggers' }))
    fireEvent.click(screen.getByRole('option', { name: 'All services' }))
    expect(h.filters.at(-1)).toEqual({ query: '', triggerKind: null, service: null })
  })

  it('offers services from the loaded flows and keeps the chosen one after the list narrows', () => {
    render(<DataFlowLens {...props} />)
    expect(screen.getByRole('option', { name: 'orders' })).toBeTruthy()
    fireEvent.click(screen.getByRole('option', { name: 'orders' }))
    h.list = { ...(h.list as object), flows: [] }
    fireEvent.click(screen.getByRole('option', { name: 'gRPC' }))
    expect(screen.getByTestId('trigger-Service').dataset.value).toBe('orders')
  })

  it('collapses the flow list into a Select below 720 px', () => {
    const { container } = render(<DataFlowLens {...props} />)
    const section = container.querySelector('section') as HTMLElement
    expect(section.dataset.compact).toBe('false')
    expect(screen.queryByTestId('trigger-Flows')).toBeNull()
    act(() => h.resize?.(600))
    expect(section.dataset.compact).toBe('true')
    expect(screen.getByTestId('trigger-Flows')).toBeTruthy()
    expect(screen.getByRole('list', { hidden: true }).hidden).toBe(true)
    fireEvent.click(screen.getByRole('option', { name: /Flow b · partial/ }))
    expect(h.setFlowId).toHaveBeenCalledWith('w', 'b')
    act(() => h.resize?.(900))
    expect(section.dataset.compact).toBe('false')
  })
})
