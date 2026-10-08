// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useGraphLens = vi.fn()
vi.mock('../../hooks/useGraphLens', () => ({ useGraphLens: (...a: unknown[]) => useGraphLens(...a) }))
vi.mock('../../hooks/usePlanTree', () => ({ usePlanTree: () => ({ tree: null, approvals: [], isLoading: false, error: null, truncated: false, refetch: vi.fn() }) }))
vi.mock('../../hooks/useTaskDependencyEdges', () => ({ useTaskDependencyEdges: () => ({ edges: new Map(), loading: false, error: false, refetch: vi.fn() }) }))
vi.mock('../../hooks/usePlanHeatmap', () => ({ usePlanHeatmap: () => ({ heatmap: null, unsupported: false }) }))
vi.mock('../../runtime/request-rpc-client', () => ({ callRequestRpc: vi.fn().mockResolvedValue({ ok: true, value: {} }) }))
vi.mock('./GraphCanvas', () => ({
  default: (p: { payload: { nodes: unknown[] }; onSelect: (id: string) => void }) => (
    <div data-testid="mock-canvas" onClick={() => p.onSelect('b')}>{p.payload.nodes.length} nodes</div>
  )
}))

import { GraphPanel } from './GraphPanel'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'
import type { OrcaRequest } from '../../../../shared/request-types'

const request = { id: 'r1', type: 'bug', size: 'M', status: 'analyzing', projectId: 'p', planTaskId: undefined } as unknown as OrcaRequest
const subject = { type: 'plan' as const, id: 'p1' }

function ready(payload: ReturnType<typeof gPayload>, extra: Record<string, unknown> = {}) {
  useGraphLens.mockReturnValue({ payload, status: 'ready', error: null, showSkeleton: false, refetch: vi.fn(), ...extra })
}

beforeEach(() => {
  useGraphLens.mockReset()
  window.matchMedia = ((q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} })) as never
  ready(gPayload([gNode('a'), gNode('b')], [gEdge('a', 'b', { change: 'added' })]))
})
afterEach(cleanup)

describe('GraphPanel', () => {
  it('starts on flow, builds it on the client and disables lenses with a reason', async () => {
    render(<GraphPanel request={request} subject={subject} impactAssessed={false} />)
    expect(await screen.findByTestId('mock-canvas')).toBeInTheDocument()
    expect(useGraphLens.mock.calls.at(-1)?.[0]).toMatchObject({ lens: 'flow', enabled: false })
    expect(screen.getByRole('radio', { name: 'Impact' })).toBeDisabled()
    expect(screen.getByRole('radio', { name: 'Plan' })).toBeDisabled()
    expect(screen.getByRole('radio', { name: 'Flow' })).not.toBeDisabled()
  })

  it('switching to a backend lens calls the hook with that lens; before/after toggle only with a change axis', async () => {
    render(<GraphPanel request={request} subject={subject} impactAssessed lensInitial="flow" />)
    expect(screen.queryByRole('radio', { name: 'Before' })).toBeNull()
    fireEvent.click(screen.getByRole('radio', { name: 'Impact' }))
    await waitFor(() => expect(useGraphLens.mock.calls.at(-1)?.[0]).toMatchObject({ lens: 'impact', enabled: true, subjectId: 'p1' }))
    expect(screen.getByRole('radio', { name: 'Before' })).toBeInTheDocument()
  })

  it('defaults to the list when the payload is truncated and shows the banner', async () => {
    ready({ ...gPayload([gNode('a')]), truncated: true, totalNodes: 900, stale: true }, {})
    render(<GraphPanel request={request} subject={subject} lensInitial="impact" impactAssessed />)
    expect(await screen.findByTestId('graph-list')).toBeInTheDocument()
    expect(screen.getByText('Showing 1 of 900 nodes')).toBeInTheDocument()
    expect(screen.getByText(/out of date/)).toBeInTheDocument()
  })

  it('shows a no-assessment state with a run button, but not when unsupported', () => {
    useGraphLens.mockReturnValue({ payload: null, status: 'idle', error: null, showSkeleton: false, refetch: vi.fn() })
    const { unmount } = render(<GraphPanel request={request} subject={subject} lensInitial="impact" />)
    expect(screen.getByText('Run assessment')).toBeInTheDocument()
    unmount()
    useGraphLens.mockReturnValue({ payload: null, status: 'idle', error: { kind: 'unsupported', code: 'x', message: 'm' }, showSkeleton: false, refetch: vi.fn() })
    render(<GraphPanel request={request} subject={subject} lensInitial="impact" />)
    expect(screen.queryByText('Run assessment')).toBeNull()
  })

  it('opens the search palette with "/" but not while typing in an input', async () => {
    render(<GraphPanel request={request} subject={subject} lensInitial="impact" impactAssessed />)
    const input = document.createElement('input')
    document.body.appendChild(input)
    fireEvent.keyDown(input, { key: '/' })
    expect(screen.queryByPlaceholderText('Search nodes...')).toBeNull()
    fireEvent.keyDown(document.body, { key: '/' })
    expect(await screen.findByPlaceholderText('Search nodes...')).toBeInTheDocument()
  })

  it('shows a retry banner on network error and keeps the previous payload', () => {
    ready(gPayload([gNode('a')]), { status: 'error', error: { kind: 'network', code: 'NETWORK_ERROR', message: 'x' } })
    render(<GraphPanel request={request} subject={subject} lensInitial="impact" impactAssessed />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('Retry')).toBeInTheDocument()
    expect(screen.getByTestId('graph-panel')).toBeInTheDocument()
  })
})
