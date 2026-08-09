// @vitest-environment happy-dom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DevServer } from '../../../../../shared/dev-server-types'
import type { TraceEvent } from '../../../../../shared/trace'

const mockState = vi.hoisted(() => ({
  removeDevServer: vi.fn(),
  traceEvents: [] as TraceEvent[],
}))

vi.mock('../../../store', () => ({
  useAppStore: (selector: (state: typeof mockState) => unknown) => selector(mockState),
}))

import { DevServerCard } from '../DevServerCard'

let root: Root | null = null
let container: HTMLDivElement | null = null

afterEach(() => {
  if (root) {
    act(() => root?.unmount())
  }
  container?.remove()
  root = null
  container = null
  mockState.traceEvents = []
})

function makeServer(overrides: Partial<DevServer> = {}): DevServer {
  return {
    id: 'ds-1',
    name: 'MacBook Pro M3',
    connectionType: 'relay-websocket',
    status: 'connected',
    platform: 'darwin',
    arch: 'arm64',
    nodeVersion: 'v20.0.0',
    lastConnectedAt: null,
    lastError: null,
    workspaceDir: null,
    addedAt: 0,
    capabilities: null,
    ...overrides,
  }
}

function makeEvent(overrides: Partial<TraceEvent> & { flow: string }): TraceEvent {
  return {
    id: 'evt-1',
    level: 'ok',
    fields: {},
    ts: 1000,
    ...overrides,
  }
}

function renderCard(server: DevServer) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => {
    root?.render(<DevServerCard server={server} showActions={false} />)
  })
  return container
}

describe('DevServerCard agent-ws trace badge', () => {
  it('renders no Agent WS badge when there is no matching trace event', () => {
    mockState.traceEvents = []
    const el = renderCard(makeServer())

    expect(el.textContent).not.toContain('Agent WS:')
  })

  it('renders "Agent WS: handshake ok" when the latest matching event level is ok', () => {
    mockState.traceEvents = [
      makeEvent({ flow: 'agentWs:handshake', level: 'ok', fields: { devServerId: 'ds-1' } }),
    ]
    const el = renderCard(makeServer({ id: 'ds-1' }))

    expect(el.textContent).toContain('Agent WS: handshake ok')
  })

  it('renders "Agent WS: <reason>" with destructive styling when the latest matching event level is fail', () => {
    mockState.traceEvents = [
      makeEvent({
        flow: 'agentWs:handshake',
        level: 'fail',
        fields: { devServerId: 'ds-1', reason: 'token expired' },
      }),
    ]
    const el = renderCard(makeServer({ id: 'ds-1' }))

    expect(el.textContent).toContain('Agent WS: token expired')
    const badge = [...el.querySelectorAll('span')].find((s) =>
      s.textContent?.includes('Agent WS: token expired')
    )
    expect(badge?.className).toContain('text-destructive')
  })
})
